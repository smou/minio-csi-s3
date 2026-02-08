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
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/smou/k8s-csi-s3/pkg/config"
	"github.com/smou/k8s-csi-s3/pkg/driver/mount"
	"github.com/smou/k8s-csi-s3/pkg/driver/nodeserver"
	"github.com/smou/k8s-csi-s3/pkg/driver/store"
	"github.com/smou/k8s-csi-s3/pkg/driver/store/minio"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"
)

const (
	unixSocketPerm                  = os.FileMode(0700) // only owner can write and read.
	grpcServerMaxReceiveMessageSize = 1024 * 1024 * 2   // 2MB
)

type Driver struct {
	Config *config.DriverConfig
	Srv    *grpc.Server
}

func NewDriver(config *config.DriverConfig) (*Driver, error) {
	klog.Infof("Initializing CSI Driver...")
	config.LogVersionInfo()

	if config.Endpoint == "" {
		return nil, fmt.Errorf("CSI endpoint not set")
	}
	if config.NodeID == "" {
		return nil, fmt.Errorf("nodeID not set")
	}
	// TODO mounter erstellen
	// mpMounter := mpmounter.New()

	return &Driver{
		Config: config,
	}, nil
}

func (d *Driver) Run() error {
	klog.Infof("Starting CSI driver at %s", d.Config.Endpoint)
	scheme, addr, err := parseEndpoint(d.Config.Endpoint)
	if err != nil {
		return err
	}

	// if scheme == "unix" {
	// 	if err := os.RemoveAll(addr); err != nil {
	// 		klog.Errorf("failed to remove socket %s: %v", addr, err)
	// 	}
	// 	// Go's `net` package does not support specifying permissions on Unix sockets it creates.
	// 	// There are two ways to change permissions:
	// 	// 	 - Using `syscall.Umask` before `net.Listen`
	// 	//   - Calling `os.Chmod` after `net.Listen`
	// 	// The first one is not nice because it affects all files created in the process,
	// 	// the second one has a time-window where the permissions of Unix socket would depend on `umask`
	// 	// between `net.Listen` and `os.Chmod`. Since we don't start accepting connections on the socket until
	// 	// `grpc.Serve` call, we should be fine with `os.Chmod` option.
	// 	// See https://github.com/golang/go/issues/11822#issuecomment-123850227.
	// 	if err := os.Chmod(addr, unixSocketPerm); err != nil {
	// 		klog.Errorf("Failed to change permissions on unix socket %s: %v", addr, err)
	// 		return fmt.Errorf("Failed to change permissions on unix socket %s: %v", addr, err)
	// 	}
	// }

	listener, err := net.Listen(scheme, addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", d.Config.Endpoint, err)
	}
	klog.Infof("Listening for connections on address: %#v", listener.Addr())
	// if scheme == "unix" {
	// 	// Go's `net` package does not support specifying permissions on Unix sockets it creates.
	// 	// There are two ways to change permissions:
	// 	// 	 - Using `syscall.Umask` before `net.Listen`
	// 	//   - Calling `os.Chmod` after `net.Listen`
	// 	// The first one is not nice because it affects all files created in the process,
	// 	// the second one has a time-window where the permissions of Unix socket would depend on `umask`
	// 	// between `net.Listen` and `os.Chmod`. Since we don't start accepting connections on the socket until
	// 	// `grpc.Serve` call, we should be fine with `os.Chmod` option.
	// 	// See https://github.com/golang/go/issues/11822#issuecomment-123850227.
	// 	if err := os.Chmod(addr, unixSocketPerm); err != nil {
	// 		klog.Errorf("Failed to change permissions on unix socket %s: %v", addr, err)
	// 		return fmt.Errorf("Failed to change permissions on unix socket %s: %v", addr, err)
	// 	}
	// }
	klog.Infof("Initializing components...")
	s3Mounter := mount.NewS3MountUtil(d.Config.MountBinaryS3)
	unixMounter := mount.NewUnixMountUtil(d.Config.MountBinary)
	store, err := minio.NewStore(&store.StoreConfig{
		EndpointURL: d.Config.S3.Endpoint,
		Region:      d.Config.S3.Region,
		AccessKey:   d.Config.S3Credentials.AccessKey,
		SecretKey:   d.Config.S3Credentials.SecretKey,
	})
	if err != nil {
		return fmt.Errorf("Error creating BucketStore: %w", err)
	}
	identityServer := NewIdentityServer(d.Config.Meta)
	controllerServer := NewControllerServer(d.Config, store)
	nodeServer := nodeserver.NewNodeServer(d.Config, unixMounter, s3Mounter)

	logErr := func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			klog.Errorf("GRPC error: %v", err)
		}
		return resp, err
	}
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(logErr),
		grpc.MaxRecvMsgSize(grpcServerMaxReceiveMessageSize),
	}
	d.Srv = grpc.NewServer(opts...)

	csi.RegisterIdentityServer(d.Srv, identityServer)
	csi.RegisterControllerServer(d.Srv, controllerServer)
	csi.RegisterNodeServer(d.Srv, nodeServer)

	klog.Infof("CSI Driver Ready!")
	return d.Srv.Serve(listener)
}

func (d *Driver) Stop() {
	if d.Srv != nil {
		klog.Info("Stopping CSI driver")
		d.Srv.GracefulStop()
	}
}

func parseEndpoint(endpoint string) (string, string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", "", fmt.Errorf("could not parse endpoint: %v", err)
	}

	addr := path.Join(u.Host, filepath.FromSlash(u.Path))

	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "tcp":
	case "unix":
		addr = path.Join("/", addr)
		if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
			return "", "", fmt.Errorf("could not remove unix domain socket %q: %v", addr, err)
		}
	default:
		return "", "", fmt.Errorf("unsupported protocol: %s", scheme)
	}

	return scheme, addr, nil
}
