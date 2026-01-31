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

	NodeID    string
	Endpoint  string
	AccessKey string
	SecretKey string
}

func NewNodeServer(config *config.DriverConfig, mountProvider mount.Provider) *NodeServer {
	return &NodeServer{
		mount:     mountProvider,
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

	if n.AccessKey == "" || n.SecretKey == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid credentials secret")
	}

	mreq := mount.MountRequest{
		TargetPath: req.TargetPath,

		Bucket:   req.GetVolumeId(),
		Endpoint: n.Endpoint,
		Region:   req.VolumeContext["region"],

		AccessKey: n.AccessKey,
		SecretKey: n.SecretKey,

		ReadOnly: req.Readonly,
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

func (n *NodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	klog.V(4).Infof("NodeStageVolume: called with args %+v", req)
	if req.GetStagingTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "stagingTargetPath missing")
	}
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "volumeId missing")
	}

	mounted, err := n.mount.IsMounted(req.StagingTargetPath)
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

	mreq := mount.MountRequest{
		TargetPath: req.StagingTargetPath,

		Bucket:   req.VolumeId,
		Endpoint: n.Endpoint,
		Region:   region,

		AccessKey: n.AccessKey,
		SecretKey: n.SecretKey,

		ReadOnly: false,
		Options:  req.VolumeContext,
	}

	if err := n.mount.Mount(ctx, mreq); err != nil {
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

	mounted, err := n.mount.IsMounted(req.StagingTargetPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	if !mounted {
		return &csi.NodeUnstageVolumeResponse{}, nil
	}

	if err := n.mount.Unmount(ctx, req.StagingTargetPath); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}

	klog.V(1).Infof("volume %s unstaged from %s", req.VolumeId, req.StagingTargetPath)

	return &csi.NodeUnstageVolumeResponse{}, nil
}
