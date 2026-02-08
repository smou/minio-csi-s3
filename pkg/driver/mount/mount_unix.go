package mount

import (
	"context"
	"fmt"
	"os"

	"k8s.io/klog/v2"
	"k8s.io/mount-utils"
)

type UnixMountUtil struct {
	Mounter mount.Interface
	// Pfad zum S3-Mount Binary (z. B. mountpoint-s3)
	Binary string
}

func NewUnixMountUtil(binary string) *UnixMountUtil {
	klog.Infof("Init Unix Mounter at %s", binary)
	return &UnixMountUtil{
		Mounter: mount.New(""),
		Binary:  binary,
	}
}

func (p *UnixMountUtil) IsMounted(targetPath string) (bool, error) {
	klog.V(4).Infof("Unix Mountutil IsMounted: called with targetPath %s", targetPath)
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		klog.ErrorS(err, "Unix Mountutil IsMounted: targetPath %s does not exist", targetPath)
		return false, nil
	}

	notMounted, err := p.Mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil {
		return false, err
	}

	return !notMounted, nil
}

func (p *UnixMountUtil) Mount(ctx context.Context, req MountRequest) error {
	klog.V(4).Infof("Unix Mountutil Mount: called with args %+v", req)
	if err := ensureDir(req.TargetPath); err != nil {
		return err
	}

	mounted, err := p.IsMounted(req.TargetPath)
	if err != nil {
		return err
	}
	if mounted {
		return nil
	}
	mounted, err = p.IsMounted(req.StagingTargetPath)
	if err != nil {
		klog.ErrorS(err, "Unix Mountutil IsMounted: staging targetPath %s does not exist", req.StagingTargetPath)
		return err
	}

	options := []string{
		"--bind",
	}
	// --allow-delete Allow delete operations on file system
	// --allow-overwrite Allow overwrite operations on file system
	if req.ReadOnly {
		options = append(options, "--read-only") // Mount file system in read-only mode
	} else {
		options = append(options, "--rw") // Mount file system in read-write mode (default)
	}
	args := []string{
		req.StagingTargetPath,
		req.TargetPath,
	}
	options = append(options, args...)
	klog.Infof("Mount options: %+v", options)
	cmd := ExecCommand(ctx, p.Binary, options...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"mount failed: %w output=%s",
			err,
			string(out),
		)
	}
	klog.Infof("%s", string(out))

	return nil
}

func (p *UnixMountUtil) Unmount(ctx context.Context, targetPath string) error {
	klog.V(4).Infof("S3 Mountutil Unmount: called with targetPath %s", targetPath)
	mounted, err := p.IsMounted(targetPath)
	if err != nil {
		return err
	}
	if !mounted {
		return nil
	}

	return p.Mounter.Unmount(targetPath)
}
