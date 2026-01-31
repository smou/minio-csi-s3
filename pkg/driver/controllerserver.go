/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package driver

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"

	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/smou/k8s-csi-s3/pkg/config"
	"github.com/smou/k8s-csi-s3/pkg/driver/store"
)

type ControllerServer struct {
	csi.UnimplementedControllerServer

	Store        store.BucketStore
	Endpoint     string
	Region       string
	BucketPrefix string
}

func NewControllerServer(config *config.DriverConfig, store store.BucketStore) *ControllerServer {
	klog.Infof("Initializing ControllerServer...")
	return &ControllerServer{
		Store:        store,
		Endpoint:     config.S3.Endpoint,
		Region:       config.S3.Region,
		BucketPrefix: config.S3.BucketPrefix,
	}
}

func (srv *ControllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	klog.V(4).Infof("CreateVolume: called with args %#v", req)

	volumeName := req.GetName()
	if volumeName == "" {
		return nil, status.Error(codes.InvalidArgument, "volume name missing")
	}

	if len(req.GetVolumeCapabilities()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume capabilities missing")
	}

	capacityBytes := int64(req.GetCapacityRange().GetRequiredBytes())
	volumeID := sanitizeVolumeID(req.GetName())
	bucketName := volumeID
	if srv.BucketPrefix != "" {
		bucketName = fmt.Sprintf("%s-%s", srv.BucketPrefix, volumeID)
	}

	// Check arguments
	if len(bucketName) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Name missing in request")
	}

	klog.Infof("Got a request to create volume %s", bucketName)

	if err := srv.Store.CreateBucket(ctx, bucketName); err != nil {
		return nil, fmt.Errorf("failed to create bucket %s: %v", bucketName, err)
	}

	//TODO handle prefixes
	// DeleteVolume lacks VolumeContext, but publish&unpublish requests have it,
	// so we don't need to store additional metadata anywhere
	context := make(map[string]string)
	// for k, v := range params {
	// 	context[k] = v
	// }
	klog.V(1).Infof("Volume %s created for region %s with capacity %v", volumeID, srv.Region, capacityBytes)
	context["capacity"] = fmt.Sprintf("%v", capacityBytes)
	context["region"] = srv.Region
	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      bucketName,
			CapacityBytes: capacityBytes,
			VolumeContext: context,
		},
	}, nil
}

func (srv *ControllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	klog.V(4).Infof("DeleteVolume: called with args %#v", req)
	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return &csi.DeleteVolumeResponse{}, nil
	}

	// Check arguments
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	klog.Infof("Deleting volume %s", volumeID)

	if err := srv.Store.DeleteBucket(ctx, volumeID); err != nil && err.Error() != "The specified bucket does not exist" {
		return nil, status.Errorf(
			codes.Internal,
			"Failed to delete bucket %s: %v",
			volumeID,
			err,
		)
	}
	klog.Infof("Bucket %s removed", volumeID)
	return &csi.DeleteVolumeResponse{}, nil
}

func (srv *ControllerServer) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	klog.V(4).Infof("ControllerGetCapabilities: called with args %#v", req)
	caps := []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
	}
	var capsResponse []*csi.ControllerServiceCapability
	for _, cap := range caps {
		c := &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: cap,
				},
			},
		}
		capsResponse = append(capsResponse, c)
	}
	return &csi.ControllerGetCapabilitiesResponse{Capabilities: capsResponse}, nil
}

func (srv *ControllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	klog.V(4).Infof("ValidateVolumeCapabilities: called with args %#v", req)

	for _, cap := range req.VolumeCapabilities {
		if cap.GetMount() == nil {
			return &csi.ValidateVolumeCapabilitiesResponse{
				Message: "only filesystem volumes supported",
			}, nil
		}
	}

	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: req.VolumeCapabilities,
		},
	}, nil
}

func sanitizeVolumeID(volumeID string) string {
	volumeID = strings.ToLower(volumeID)
	if len(volumeID) > 63 {
		h := sha1.New()
		io.WriteString(h, volumeID)
		volumeID = hex.EncodeToString(h.Sum(nil))
	}
	return volumeID
}
