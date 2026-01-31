package mount

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"k8s.io/klog/v2"
	"k8s.io/mount-utils"
)

var ExecCommand = exec.CommandContext

type MountUtilsProvider struct {
	Mounter mount.Interface
	// Pfad zum S3-Mount Binary (z. B. mountpoint-s3)
	Binary string
}

func NewMountUtilsProvider(binary string) *MountUtilsProvider {
	klog.Infof("Init Mounter at %s", binary)
	return &MountUtilsProvider{
		Mounter: mount.New(""),
		Binary:  binary,
	}
}

func (p *MountUtilsProvider) IsMounted(targetPath string) (bool, error) {
	klog.V(4).Infof("k8s-Mountutil IsMounted: called with targetPath %s", targetPath)
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		klog.V(4).ErrorS(err, "k8s-Mountutil IsMounted: targetPath %s does not exist", targetPath)
		return false, nil
	}

	notMounted, err := p.Mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil {
		return false, err
	}

	return !notMounted, nil
}

func (p *MountUtilsProvider) Mount(ctx context.Context, req MountRequest) error {
	klog.V(4).Infof("k8s-Mountutil Mount: called with args %+v", req)
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

	options := []string{
		"--endpoint-url", req.Endpoint,
		"--region", req.Region,
		"--force-path-style",   // Force path-style addressing
		"--incremental-upload", // Enable incremental uploads and support for appending to existing objects
		"--allow-other",        // FUSE option to Allow other users, including root, to access file system
	}
	if req.GID != "" {
		options = append(options,
			"--gid", req.GID, // Owner GID [default: current user's GID]
			"--dir-mode", "0775", // Set the permissions for directories (default: 0775)
			"--file-mode", "0664", // Set the permissions for files (default: 0664)
		)
	}
	// --allow-delete Allow delete operations on file system
	// --allow-overwrite Allow overwrite operations on file system
	if req.ReadOnly {
		options = append(options, "--read-only") // Mount file system in read-only mode
	}
	args := []string{
		req.Bucket,
		req.TargetPath,
	}
	options = append(options, args...)
	klog.Infof("Mount options: %+v", options)
	cmd := ExecCommand(ctx, p.Binary, options...)

	// Credentials über ENV (best practice)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", req.AccessKey),
		fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", req.SecretKey),
	)

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

func (p *MountUtilsProvider) Unmount(ctx context.Context, targetPath string) error {
	klog.V(4).Infof("k8s-Mountutil Unmount: called with targetPath %s", targetPath)
	mounted, err := p.IsMounted(targetPath)
	if err != nil {
		return err
	}
	if !mounted {
		return nil
	}

	return p.Mounter.Unmount(targetPath)
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0755)
}
