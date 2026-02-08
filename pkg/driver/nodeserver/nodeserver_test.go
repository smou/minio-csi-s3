package nodeserver_test

import (
	"context"
	"errors"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/smou/k8s-csi-s3/pkg/config"
	"github.com/smou/k8s-csi-s3/pkg/driver/mount"
	"github.com/smou/k8s-csi-s3/pkg/driver/nodeserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestNodeServer(mount *FakeMountProvider) *nodeserver.NodeServer {
	cfg := &config.DriverConfig{
		NodeID: "node-1",
		S3: config.S3Config{
			Endpoint: "https://minio.local",
		},
		S3Credentials: config.S3Credentials{
			AccessKey: "access",
			SecretKey: "secret",
		},
	}

	return nodeserver.NewNodeServer(cfg, mount, mount)
}

func TestNodeGetInfo(t *testing.T) {
	ns := newTestNodeServer(NewFakeMountProvider())

	resp, err := ns.NodeGetInfo(context.Background(), nil)
	assert.NoError(t, err)
	assert.Equal(t, "node-1", resp.NodeId)
}

func TestNodeGetCapabilities(t *testing.T) {
	ns := newTestNodeServer(NewFakeMountProvider())

	resp, err := ns.NodeGetCapabilities(context.Background(), nil)
	assert.NoError(t, err)
	assert.Len(t, resp.Capabilities, 2)
}

func TestNodePublishVolume_Success(t *testing.T) {
	mount := NewFakeMountProvider()
	ns := newTestNodeServer(mount)

	req := &csi.NodePublishVolumeRequest{
		VolumeId:          "test-bucket",
		StagingTargetPath: "/mnt/stage",
		TargetPath:        "/mnt/test",
		VolumeContext: map[string]string{
			"region": "us-east-1",
		},
	}

	resp, err := ns.NodePublishVolume(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	mounted, _ := mount.IsMounted("/mnt/test")
	assert.True(t, mounted)

}

func TestNodePublishVolume_Idempotent(t *testing.T) {
	mountProvider := NewFakeMountProvider()
	ns := newTestNodeServer(mountProvider)

	// Vorab mounten
	_ = mountProvider.Mount(context.Background(), mount.MountRequest{
		StagingTargetPath: "/mnt/stage",
		TargetPath:        "/mnt/test",
	})

	req := &csi.NodePublishVolumeRequest{
		VolumeId:          "test-bucket",
		StagingTargetPath: "/mnt/stage",
		TargetPath:        "/mnt/test",
		VolumeContext: map[string]string{
			"region": "us-east-1",
		},
	}

	resp, err := ns.NodePublishVolume(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestNodePublishVolume_MissingStageTargetPath(t *testing.T) {
	mount := NewFakeMountProvider()
	ns := newTestNodeServer(mount)

	req := &csi.NodePublishVolumeRequest{
		VolumeId:   "test-bucket",
		TargetPath: "/mnt/test",
		VolumeContext: map[string]string{
			"region": "us-east-1",
		},
	}

	_, err := ns.NodePublishVolume(context.Background(), req)
	assert.Error(t, err)
}

func TestNodeUnpublishVolume_Success(t *testing.T) {
	mountProvider := NewFakeMountProvider()
	ns := newTestNodeServer(mountProvider)

	_ = mountProvider.Mount(context.Background(), mount.MountRequest{
		TargetPath: "/mnt/test",
	})

	req := &csi.NodeUnpublishVolumeRequest{
		TargetPath: "/mnt/test",
	}

	resp, err := ns.NodeUnpublishVolume(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	mounted, _ := mountProvider.IsMounted("/mnt/test")
	assert.False(t, mounted)
}

func TestNodeUnpublishVolume_Idempotent(t *testing.T) {
	ns := newTestNodeServer(NewFakeMountProvider())

	req := &csi.NodeUnpublishVolumeRequest{
		TargetPath: "/mnt/does-not-exist",
	}

	resp, err := ns.NodeUnpublishVolume(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestNodeStageVolume_Success(t *testing.T) {
	mp := NewFakeMountProvider()
	ns := newTestNodeServer(mp)

	req := &csi.NodeStageVolumeRequest{
		VolumeId:          "bucket-1",
		StagingTargetPath: "/staging/path",
		VolumeContext: map[string]string{
			"region": "us-east-1",
		},
	}

	_, err := ns.NodeStageVolume(context.Background(), req)
	require.NoError(t, err)

	mounted, _ := mp.IsMounted("/staging/path")
	require.True(t, mounted)
}

func TestNodeStageVolume_Idempotent(t *testing.T) {
	mp := NewFakeMountProvider()
	mp.mounted["/staging/path"] = true

	ns := newTestNodeServer(mp)

	req := &csi.NodeStageVolumeRequest{
		VolumeId:          "bucket-1",
		StagingTargetPath: "/staging/path",
	}

	_, err := ns.NodeStageVolume(context.Background(), req)
	require.NoError(t, err)
}

func TestNodeStageVolume_MountError(t *testing.T) {
	mp := NewFakeMountProvider()
	mp.mountErr = errors.New("mount failed")

	ns := newTestNodeServer(mp)

	req := &csi.NodeStageVolumeRequest{
		VolumeId:          "bucket-1",
		StagingTargetPath: "/staging/path",
	}

	_, err := ns.NodeStageVolume(context.Background(), req)
	require.Error(t, err)
}

func TestNodeUnstageVolume_Success(t *testing.T) {
	mp := NewFakeMountProvider()
	mp.mounted["/staging/path"] = true

	ns := newTestNodeServer(mp)

	req := &csi.NodeUnstageVolumeRequest{
		VolumeId:          "bucket-1",
		StagingTargetPath: "/staging/path",
	}

	_, err := ns.NodeUnstageVolume(context.Background(), req)
	require.NoError(t, err)

	mounted, _ := mp.IsMounted("/staging/path")
	require.False(t, mounted)
}

func TestNodeUnstageVolume_Idempotent(t *testing.T) {
	mp := NewFakeMountProvider()
	ns := newTestNodeServer(mp)

	req := &csi.NodeUnstageVolumeRequest{
		VolumeId:          "bucket-1",
		StagingTargetPath: "/staging/path",
	}

	_, err := ns.NodeUnstageVolume(context.Background(), req)
	require.NoError(t, err)
}

func TestNodeUnstageVolume_UnmountError(t *testing.T) {
	mp := NewFakeMountProvider()
	mp.mounted["/staging/path"] = true
	mp.unmountErr = errors.New("unmount failed")

	ns := newTestNodeServer(mp)

	req := &csi.NodeUnstageVolumeRequest{
		VolumeId:          "bucket-1",
		StagingTargetPath: "/staging/path",
	}

	_, err := ns.NodeUnstageVolume(context.Background(), req)
	require.Error(t, err)
}
