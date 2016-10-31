package commands // import "code.cloudfoundry.org/grootfs/commands"

import (
	"errors"
	"fmt"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/commands/idfinder"
	"code.cloudfoundry.org/grootfs/commands/storepath"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/bundler"
	"code.cloudfoundry.org/grootfs/store/dependency_manager"
	"code.cloudfoundry.org/grootfs/store/volume_driver"
	"code.cloudfoundry.org/lager"

	"github.com/urfave/cli"
)

var DeleteCommand = cli.Command{
	Name:        "delete",
	Usage:       "delete <id|bundle path>",
	Description: "Deletes a container bundle",

	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)
		logger = logger.Session("delete")

		storePath := ctx.GlobalString("store")
		if ctx.NArg() != 1 {
			logger.Error("parsing-command", errors.New("id was not specified"))
			return cli.NewExitError("id was not specified", 1)
		}

		idOrPath := ctx.Args().First()
		id, err := idfinder.FindID(storePath, idOrPath)
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}

		if id == idOrPath {
			storePath = storepath.UserBased(storePath)
		} else {
			storePath, err = idfinder.FindSubStorePath(storePath, idOrPath)
			if err != nil {
				return cli.NewExitError(fmt.Sprintf("can't determine the store path: %s", err.Error()), 1)
			}
		}

		btrfsVolumeDriver := volume_driver.NewBtrfs(ctx.GlobalString("drax-bin"), storePath)
		bundler := bundler.NewBundler(btrfsVolumeDriver, storePath)
		dependencyManager := dependency_manager.NewDependencyManager(
			filepath.Join(storePath, store.META_DIR_NAME, "dependencies"),
		)
		deleter := groot.IamDeleter(bundler, dependencyManager)

		err = deleter.Delete(logger, id)
		if err != nil {
			logger.Error("deleting-bundle-failed", err)
			return cli.NewExitError(err.Error(), 1)
		}

		fmt.Printf("Bundle %s deleted\n", id)
		return nil
	},
}
