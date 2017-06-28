package runner

import (
	"encoding/json"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/onsi/gomega/gexec"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store/manager"
)

func (r Runner) StartCreate(spec groot.CreateSpec) (*gexec.Session, error) {
	if !r.skipInitStore {
		if err := r.initStoreAsRoot(); err != nil {
			return nil, err
		}
	}
	args := r.makeCreateArgs(spec)
	return r.StartSubcommand("create", args...)
}

func (r Runner) Create(spec groot.CreateSpec) (groot.ImageInfo, error) {
	if !r.skipInitStore {
		if err := r.initStoreAsRoot(); err != nil {
			return groot.ImageInfo{}, err
		}
	}

	r.Timeout = 60 * time.Second
	args := r.makeCreateArgs(spec)
	output, err := r.RunSubcommand("create", args...)
	if err != nil {
		return groot.ImageInfo{}, err
	}

	imageInfo := groot.ImageInfo{}
	json.Unmarshal([]byte(output), &imageInfo)
	imageInfo.Path = filepath.Dir(imageInfo.Rootfs)

	return imageInfo, nil
}

func (r Runner) EnsureMounted(image groot.ImageInfo) error {
	if image.Mount != nil {
		return syscall.Mount(image.Mount.Source, image.Mount.Destination, image.Mount.Type, 0, image.Mount.Options[0])
	}

	return nil
}

func (r Runner) initStoreAsRoot() error {
	spec := manager.InitSpec{}

	if r.SysCredential.Uid != 0 {
		spec.UIDMappings = defaultIdMapping(r.SysCredential.Uid)
		spec.GIDMappings = defaultIdMapping(r.SysCredential.Gid)
	}

	if err := r.RunningAsUser(0, 0).InitStore(spec); err != nil {
		return err
	}

	return nil
}

func defaultIdMapping(hostId uint32) []groot.IDMappingSpec {
	return []groot.IDMappingSpec{
		groot.IDMappingSpec{
			HostID:      int(hostId),
			NamespaceID: 0,
			Size:        1,
		},
		{HostID: 100000, NamespaceID: 1, Size: 65000},
	}
}

func (r Runner) makeCreateArgs(spec groot.CreateSpec) []string {
	args := []string{}

	if r.CleanOnCreate || r.NoCleanOnCreate {
		if r.CleanOnCreate {
			args = append(args, "--with-clean")
		}
		if r.NoCleanOnCreate {
			args = append(args, "--without-clean")
		}
	} else {
		if spec.CleanOnCreate {
			args = append(args, "--with-clean")
		} else {
			args = append(args, "--without-clean")
		}
	}

	if spec.Mount {
		args = append(args, "--with-mount")
	} else {
		args = append(args, "--without-mount")
	}

	if r.InsecureRegistry != "" {
		args = append(args, "--insecure-registry", r.InsecureRegistry)
	}

	if r.RegistryUsername != "" {
		args = append(args, "--username", r.RegistryUsername)
	}

	if r.RegistryPassword != "" {
		args = append(args, "--password", r.RegistryPassword)
	}

	if spec.DiskLimit != 0 {
		args = append(args, "--disk-limit-size-bytes",
			strconv.FormatInt(spec.DiskLimit, 10),
		)
		if spec.ExcludeBaseImageFromQuota {
			args = append(args, "--exclude-image-from-quota")
		}
	}

	if spec.BaseImage != "" {
		args = append(args, spec.BaseImage)
	}

	if spec.ID != "" {
		args = append(args, spec.ID)
	}

	return args
}
