package commands // import "code.cloudfoundry.org/grootfs/commands"

import (
	"crypto/x509"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	unpackerpkg "code.cloudfoundry.org/grootfs/base_image_puller/unpacker"
	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/fetcher/local"
	"code.cloudfoundry.org/grootfs/fetcher/remote"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/metrics"
	"code.cloudfoundry.org/grootfs/store"
	storepkg "code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/cache_driver"
	"code.cloudfoundry.org/grootfs/store/dependency_manager"
	"code.cloudfoundry.org/grootfs/store/garbage_collector"
	"code.cloudfoundry.org/grootfs/store/image_cloner"
	locksmithpkg "code.cloudfoundry.org/grootfs/store/locksmith"
	"code.cloudfoundry.org/lager"

	"code.cloudfoundry.org/commandrunner/linux_command_runner"
	"github.com/docker/distribution/registry/api/errcode"
	errorspkg "github.com/pkg/errors"
	"github.com/urfave/cli"
)

type fileSystemDriver interface {
	image_cloner.ImageDriver
	base_image_puller.VolumeDriver
}

var CreateCommand = cli.Command{
	Name:        "create",
	Usage:       "create [options] <image> <id>",
	Description: "Creates a root filesystem for the provided image.",

	Flags: []cli.Flag{
		cli.Int64Flag{
			Name:  "disk-limit-size-bytes",
			Usage: "Inclusive disk limit (i.e: includes all layers in the filesystem)",
		},
		cli.StringSliceFlag{
			Name:  "uid-mapping",
			Usage: "UID mapping for image translation, e.g.: <Namespace UID>:<Host UID>:<Size>",
		},
		cli.StringSliceFlag{
			Name:  "gid-mapping",
			Usage: "GID mapping for image translation, e.g.: <Namespace GID>:<Host GID>:<Size>",
		},
		cli.StringSliceFlag{
			Name:  "insecure-registry",
			Usage: "Whitelist a private registry",
		},
		cli.BoolFlag{
			Name:  "exclude-image-from-quota",
			Usage: "Set disk limit to be exclusive (i.e.: excluding image layers)",
		},
		cli.BoolFlag{
			Name:  "clean",
			Usage: "Clean up unused layers before creating rootfs",
		},
		cli.BoolFlag{
			Name:  "no-clean",
			Usage: "Do NOT clean up unused layers before creating rootfs",
		},
		cli.StringFlag{
			Name:  "username",
			Usage: "Username to authenticate in image registry",
		},
		cli.StringFlag{
			Name:  "password",
			Usage: "Password to authenticate in image registry",
		},
	},

	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)
		logger = logger.Session("create")

		if ctx.NArg() != 2 {
			logger.Error("parsing-command", errors.New("invalid arguments"), lager.Data{"args": ctx.Args()})
			return cli.NewExitError(fmt.Sprintf("invalid arguments - usage: %s", ctx.Command.Usage), 1)
		}

		if ctx.IsSet("clean") && ctx.IsSet("no-clean") {
			return cli.NewExitError("clean and no-clean cannot be used together", 1)
		}

		configBuilder := ctx.App.Metadata["configBuilder"].(*config.Builder)
		configBuilder.WithInsecureRegistries(ctx.StringSlice("insecure-registry")).
			WithUIDMappings(ctx.StringSlice("uid-mapping")).
			WithGIDMappings(ctx.StringSlice("gid-mapping")).
			WithDiskLimitSizeBytes(ctx.Int64("disk-limit-size-bytes"),
				ctx.IsSet("disk-limit-size-bytes")).
			WithExcludeBaseImageFromQuota(ctx.Bool("exclude-image-from-quota"),
				ctx.IsSet("exclude-image-from-quota")).
			WithCleanOnCreate(ctx.IsSet("clean"), ctx.IsSet("no-clean"))

		cfg, err := configBuilder.Build()
		logger.Debug("create-config", lager.Data{"currentConfig": cfg})
		if err != nil {
			logger.Error("config-builder-failed", err)
			return cli.NewExitError(err.Error(), 1)
		}

		storePath := cfg.StorePath
		baseImage := ctx.Args().First()
		id := ctx.Args().Tail()[0]

		uidMappings, err := parseIDMappings(cfg.UIDMappings)
		if err != nil {
			err = fmt.Errorf("parsing uid-mapping: %s", err)
			logger.Error("parsing-command", err)
			return cli.NewExitError(err.Error(), 1)
		}
		gidMappings, err := parseIDMappings(cfg.GIDMappings)
		if err != nil {
			err = fmt.Errorf("parsing gid-mapping: %s", err)
			logger.Error("parsing-command", err)
			return cli.NewExitError(err.Error(), 1)
		}

		storeOwnerUid, storeOwnerGid, err := findStoreOwner(uidMappings, gidMappings)
		if err != nil {
			return err
		}
		if err = store.ConfigureStore(logger, storePath, storeOwnerUid, storeOwnerGid, id); err != nil {
			return err
		}

		fsDriver, err := createFileSystemDriver(cfg)
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}

		imageCloner := image_cloner.NewImageCloner(fsDriver, storePath)

		runner := linux_command_runner.New()
		var unpacker base_image_puller.Unpacker
		if os.Getuid() == 0 {
			unpacker = unpackerpkg.NewNSSysProcUnpacker(runner, cfg.FSDriver)
		} else {
			idMapper := unpackerpkg.NewIDMapper(cfg.NewuidmapBin, cfg.NewgidmapBin, runner)
			unpacker = unpackerpkg.NewNSIdMapperUnpacker(runner, idMapper, cfg.FSDriver)
		}

		dockerSrc := remote.NewDockerSource(ctx.String("username"), ctx.String("password"), cfg.InsecureRegistries)

		cacheDriver := cache_driver.NewCacheDriver(storePath)
		remoteFetcher := remote.NewRemoteFetcher(dockerSrc, cacheDriver)

		localFetcher := local.NewLocalFetcher()

		locksmith := locksmithpkg.NewFileSystem(storePath)
		dependencyManager := dependency_manager.NewDependencyManager(
			filepath.Join(storePath, storepkg.MetaDirName, "dependencies"),
		)
		baseImagePuller := base_image_puller.NewBaseImagePuller(
			localFetcher,
			remoteFetcher,
			unpacker,
			fsDriver,
			dependencyManager,
		)
		rootFSConfigurer := storepkg.NewRootFSConfigurer()
		metricsEmitter := metrics.NewEmitter()

		sm := garbage_collector.NewStoreMeasurer(storePath)
		gc := garbage_collector.NewGC(cacheDriver, fsDriver, imageCloner, dependencyManager)
		cleaner := groot.IamCleaner(locksmith, sm, gc, metricsEmitter)

		namespaceChecker := groot.NewNamespaceChecker(storePath)

		creator := groot.IamCreator(
			imageCloner, baseImagePuller, locksmith, rootFSConfigurer,
			dependencyManager, metricsEmitter, cleaner, namespaceChecker,
		)

		createSpec := groot.CreateSpec{
			ID:                          id,
			BaseImage:                   baseImage,
			DiskLimit:                   cfg.DiskLimitSizeBytes,
			ExcludeBaseImageFromQuota:   cfg.ExcludeBaseImageFromQuota,
			UIDMappings:                 uidMappings,
			GIDMappings:                 gidMappings,
			CleanOnCreate:               cfg.CleanOnCreate,
			CleanOnCreateThresholdBytes: cfg.CleanThresholdBytes,
			CleanOnCreateIgnoreImages:   cfg.IgnoreBaseImages,
		}
		image, err := creator.Create(logger, createSpec)
		if err != nil {
			logger.Error("creating", err)

			humanizedError := tryHumanize(err, createSpec)
			return cli.NewExitError(humanizedError, 1)
		}

		fmt.Println(image.Path)
		return nil
	},
}

func parseIDMappings(args []string) ([]groot.IDMappingSpec, error) {
	mappings := []groot.IDMappingSpec{}

	for _, v := range args {
		var mapping groot.IDMappingSpec
		_, err := fmt.Sscanf(v, "%d:%d:%d", &mapping.NamespaceID, &mapping.HostID, &mapping.Size)
		if err != nil {
			return nil, err
		}
		mappings = append(mappings, mapping)
	}

	return mappings, nil
}

func containsDockerError(errorsList errcode.Errors, errCode errcode.ErrorCode) bool {
	for _, err := range errorsList {
		if e, ok := err.(errcode.Error); ok && e.ErrorCode() == errCode {
			return true
		}
	}

	return false
}

func tryHumanizeDockerErrorsList(err errcode.Errors, spec groot.CreateSpec) string {
	if containsDockerError(err, errcode.ErrorCodeUnauthorized) {
		return fmt.Sprintf("%s does not exist or you do not have permissions to see it.", spec.BaseImage)
	}

	return err.Error()
}

func tryParsingErrorMessage(err error) error {
	if err.Error() == "unable to retrieve auth token: 401 unauthorized" {
		return errors.New("authorization failed: username and password are invalid")
	}

	return err
}

func tryHumanize(err error, spec groot.CreateSpec) string {
	switch e := errorspkg.Cause(err).(type) {
	case *url.Error:
		if _, ok := e.Err.(x509.UnknownAuthorityError); ok {
			return "This registry is insecure. To pull images from this registry, please use the --insecure-registry option."
		}

	case errcode.Errors:
		return tryHumanizeDockerErrorsList(e, spec)
	}

	return tryParsingErrorMessage(errorspkg.Cause(err)).Error()
}

func findStoreOwner(uidMappings, gidMappings []groot.IDMappingSpec) (int, int, error) {
	uid := os.Getuid()
	gid := os.Getgid()

	for _, mapping := range uidMappings {
		if mapping.Size == 1 && mapping.NamespaceID == 0 {
			uid = mapping.HostID
			break
		}
		uid = -1
	}

	if len(uidMappings) > 0 && uid == -1 {
		return 0, 0, errors.New("couldn't determine store owner, missing root user mapping")
	}

	for _, mapping := range gidMappings {
		if mapping.Size == 1 && mapping.NamespaceID == 0 {
			gid = mapping.HostID
			break
		}
		gid = -1
	}

	if len(gidMappings) > 0 && gid == -1 {
		return 0, 0, errors.New("couldn't determine store owner, missing root user mapping")
	}

	return uid, gid, nil
}
