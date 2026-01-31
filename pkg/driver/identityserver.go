/*
Copyright 2022 The Kubernetes Authors

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
	"github.com/smou/k8s-csi-s3/pkg/config"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"k8s.io/klog/v2"
)

type IdentityServer struct {
	csi.UnimplementedIdentityServer

	DriverName    string
	DriverVersion string
}

func NewIdentityServer(meta config.Meta) *IdentityServer {
	klog.Infof("Initializing IdentityServer...")
	return &IdentityServer{
		DriverName:    meta.DriverName,
		DriverVersion: meta.DriverVersion,
	}
}

func (srv *IdentityServer) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	resp := &csi.GetPluginInfoResponse{
		Name:          srv.DriverName,
		VendorVersion: srv.DriverVersion,
	}

	return resp, nil
}

func (srv *IdentityServer) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	caps := []csi.PluginCapability_Service_Type{
		csi.PluginCapability_Service_CONTROLLER_SERVICE,
	}
	var capsResponse []*csi.PluginCapability
	for _, cap := range caps {
		c := &csi.PluginCapability{
			Type: &csi.PluginCapability_Service_{
				Service: &csi.PluginCapability_Service{
					Type: cap,
				},
			},
		}
		capsResponse = append(capsResponse, c)
	}
	return &csi.GetPluginCapabilitiesResponse{Capabilities: capsResponse}, nil
}

func (srv *IdentityServer) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	return &csi.ProbeResponse{
		Ready: wrapperspb.Bool(true),
	}, nil
}
