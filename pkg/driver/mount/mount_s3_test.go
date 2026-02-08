package mount_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	provider "github.com/smou/k8s-csi-s3/pkg/driver/mount"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakeExecCommand(success bool) func(context.Context, string, ...string) *exec.Cmd {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if success {
			return exec.CommandContext(ctx, "true")
		}
		return exec.CommandContext(ctx, "false")
	}
}

func TestIsMounted(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "mnt")

	err := os.MkdirAll(target, 0755)
	require.NoError(t, err)

	mounter := NewFakeMounter()
	p := &provider.S3MountUtil{
		Mounter: mounter,
		Binary:  "dummy",
	}

	mounted, err := p.IsMounted(target)
	assert.NoError(t, err)
	assert.False(t, mounted)

	mounter.mounted[target] = true

	mounted, err = p.IsMounted(target)
	assert.NoError(t, err)
	assert.True(t, mounted)
}

func TestMount_Success(t *testing.T) {
	oldExec := provider.ExecCommand
	defer func() { provider.ExecCommand = oldExec }()
	provider.ExecCommand = fakeExecCommand(true)

	mounter := NewFakeMounter()
	p := &provider.S3MountUtil{
		Mounter: mounter,
		Binary:  "mountpoint-s3",
	}

	req := provider.MountRequest{
		TargetPath: "/tmp/mnt",
		Bucket:     "bucket",
		Endpoint:   "https://minio",
		Region:     "us-east-1",
		AccessKey:  "ak",
		SecretKey:  "sk",
	}

	err := p.Mount(context.Background(), req)
	assert.NoError(t, err)
}

func TestMount_Idempotent(t *testing.T) {
	oldExec := provider.ExecCommand
	defer func() { provider.ExecCommand = oldExec }()
	provider.ExecCommand = fakeExecCommand(true)

	mounter := NewFakeMounter()
	mounter.mounted["/tmp/mnt"] = true

	p := &provider.S3MountUtil{
		Mounter: mounter,
		Binary:  "mountpoint-s3",
	}

	err := p.Mount(context.Background(), provider.MountRequest{
		TargetPath: "/tmp/mnt",
	})
	assert.NoError(t, err)
}

func TestMount_Failure(t *testing.T) {
	oldExec := provider.ExecCommand
	defer func() { provider.ExecCommand = oldExec }()
	provider.ExecCommand = fakeExecCommand(false)

	mounter := NewFakeMounter()
	p := &provider.S3MountUtil{
		Mounter: mounter,
		Binary:  "mountpoint-s3",
	}

	err := p.Mount(context.Background(), provider.MountRequest{
		TargetPath: "/tmp/mnt",
		Bucket:     "bucket",
	})
	assert.Error(t, err)
}

func TestUnmount(t *testing.T) {
	mounter := NewFakeMounter()
	mounter.mounted["/mnt/test"] = true

	p := &provider.S3MountUtil{
		Mounter: mounter,
	}

	err := p.Unmount(context.Background(), "/mnt/test")
	assert.NoError(t, err)

	mounted, _ := p.IsMounted("/mnt/test")
	assert.False(t, mounted)
}
