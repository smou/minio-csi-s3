package nodeserver

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/smou/k8s-csi-s3/pkg/config"
	"github.com/smou/k8s-csi-s3/pkg/driver/mount"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

var (
	capabilities = []csi.NodeServiceCapability_RPC_Type{
		csi.NodeServiceCapability_RPC_VOLUME_MOUNT_GROUP,
		csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
	}
)

type NodeServer struct {
	csi.UnimplementedNodeServer

	mount mount.Provider
	s3    mount.Provider

	NodeID    string
	Endpoint  string
	AccessKey string
	SecretKey string
}

func NewNodeServer(config *config.DriverConfig, mountProvider mount.Provider, s3MountProvider mount.Provider) *NodeServer {
	return &NodeServer{
		mount:     mountProvider,
		s3:        s3MountProvider,
		NodeID:    config.NodeID,
		Endpoint:  config.S3.Endpoint,
		AccessKey: config.S3Credentials.AccessKey,
		SecretKey: config.S3Credentials.SecretKey,
	}
}

func (n *NodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: n.NodeID,
	}, nil
}

func (n *NodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	klog.V(4).Infof("NodeGetCapabilities: called with args %+v", req)
	// currently there is a single NodeServer capability according to the spec
	var capsResponse []*csi.NodeServiceCapability
	for _, cap := range capabilities {
		c := &csi.NodeServiceCapability{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: cap,
				},
			},
		}
		capsResponse = append(capsResponse, c)
	}

	return &csi.NodeGetCapabilitiesResponse{Capabilities: capsResponse}, nil
}

func (n *NodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	klog.V(4).Infof("NodePublishVolume: called with args %+v", req)
	if req.GetTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "targetPath missing")
	}
	if req.GetStagingTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "staging targetPath missing")
	}

	if req.GetVolumeContext() == nil {
		return nil, status.Error(codes.InvalidArgument, "volumeContext missing")
	}
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	mounted, err := n.mount.IsMounted(req.TargetPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	if mounted {
		return &csi.NodePublishVolumeResponse{}, nil
	}

	region := ""
	if req.GetVolumeContext() != nil {
		region = req.VolumeContext["region"]
	}

	mreq := mount.MountRequest{
		StagingTargetPath: req.StagingTargetPath,
		TargetPath:        req.TargetPath,

		Bucket: req.GetVolumeId(),
		Region: region,

		ReadOnly: false,
		Options:  req.VolumeContext,
	}

	if err := n.mount.Mount(ctx, mreq); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

func (n *NodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	klog.V(4).Infof("NodeUnpublishVolume: called with args %+v", req)
	if req.GetTargetPath() == "" {
		return &csi.NodeUnpublishVolumeResponse{}, nil
	}

	mounted, err := n.mount.IsMounted(req.TargetPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}

	if !mounted {
		return &csi.NodeUnpublishVolumeResponse{}, nil
	}

	if err := n.mount.Unmount(ctx, req.TargetPath); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	klog.V(1).Infof("volume %s has been unmounted.", req.VolumeId)
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

/*
Mount s3 to staging path, which will be used for publish volume.
Staging path is used to prepare the volume before it is published to the target path.
This allows for better performance and reliability when mounting the volume to the target path.
*/
func (n *NodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	klog.V(4).Infof("NodeStageVolume: called with args %+v", req)
	if req.GetStagingTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "stagingTargetPath missing")
	}
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "volumeId missing")
	}

	mounted, err := n.s3.IsMounted(req.StagingTargetPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	if mounted {
		return &csi.NodeStageVolumeResponse{}, nil
	}

	if n.AccessKey == "" || n.SecretKey == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid credentials")
	}

	region := ""
	if req.GetVolumeContext() != nil {
		region = req.VolumeContext["region"]
	}

	gid := getGIDFromVolumeCapability(req.GetVolumeCapability())

	mreq := mount.MountRequest{
		StagingTargetPath: req.StagingTargetPath,
		TargetPath:        req.StagingTargetPath,

		Bucket:   req.VolumeId,
		Endpoint: n.Endpoint,
		Region:   region,

		AccessKey: n.AccessKey,
		SecretKey: n.SecretKey,

		ReadOnly: false,
		GID:      gid,
		Options:  req.VolumeContext,
	}

	if err := n.s3.Mount(ctx, mreq); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}

	klog.V(1).Infof("volume %s staged at %s", req.VolumeId, req.StagingTargetPath)

	return &csi.NodeStageVolumeResponse{}, nil
}

func (n *NodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	klog.V(4).Infof("NodeUnstageVolume: called with args %+v", req)
	if req.GetStagingTargetPath() == "" {
		return &csi.NodeUnstageVolumeResponse{}, nil
	}

	mounted, err := n.s3.IsMounted(req.StagingTargetPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	if !mounted {
		return &csi.NodeUnstageVolumeResponse{}, nil
	}

	if err := n.s3.Unmount(ctx, req.StagingTargetPath); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}

	klog.V(1).Infof("volume %s unstaged from %s", req.VolumeId, req.StagingTargetPath)

	return &csi.NodeUnstageVolumeResponse{}, nil
}

func getGIDFromVolumeCapability(volCap *csi.VolumeCapability) string {
	if volCap != nil {
		mountCap := volCap.GetMount()
		if mountCap != nil {
			group := mountCap.GetVolumeMountGroup()
			if group != "" {
				return group
			}
		}
	}
	return ""
}
