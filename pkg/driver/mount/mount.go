package mount

import (
	"context"
	"os"
	"os/exec"
)

var ExecCommand = exec.CommandContext

type Provider interface {
	// Mount mountet das Volume auf den Zielpfad
	Mount(ctx context.Context, req MountRequest) error

	// Unmount entfernt das Mount
	Unmount(ctx context.Context, targetPath string) error

	// IsMounted prüft Idempotenz
	IsMounted(targetPath string) (bool, error)
}

type MountRequest struct {
	StagingTargetPath string
	TargetPath        string

	Bucket   string
	Endpoint string
	Region   string

	AccessKey string
	SecretKey string

	ReadOnly bool
	GID      string

	Options map[string]string
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0755)
}
