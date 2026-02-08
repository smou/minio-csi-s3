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

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/smou/k8s-csi-s3/pkg/config"
	"github.com/smou/k8s-csi-s3/pkg/driver"
	"k8s.io/klog/v2"
)

func init() {

}

var (
	endpoint      = flag.String("endpoint", "unix://csi/csi.sock", "CSI endpoint")
	nodeID        = flag.String("nodeid", "controller", "kubernetes node id")
	mountBinaryS3 = flag.String("mountBinary", "/usr/local/bin/mount-s3", "s3 mount binary path")
	mountBinary   = flag.String("mountBinary", "/usr/bin/mount", "unix mount binary path")
)

func main() {
	klog.InitFlags(nil)

	flag.Set("logtostderr", "true")
	flag.Parse()
	defer klog.Flush()

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer cancel()

	config, err := config.InitConfig(ctx)
	if err != nil {
		log.Fatalf("Error loading DriverConfig: %v", err)
	}
	config.Endpoint = *endpoint
	config.NodeID = *nodeID
	config.MountBinaryS3 = *mountBinaryS3
	config.MountBinary = *mountBinary
	if err := preflightChecks(config); err != nil {
		log.Fatalf("Preflight checks failed: %v", err)
	}
	runDriver(config, ctx)
	os.Exit(0)
}

func runDriver(config *config.DriverConfig, ctx context.Context) {
	driver, err := driver.NewDriver(config)
	if err != nil {
		log.Fatalf("Error init Driver: %v", err)
	}
	go func() {
		if err := driver.Run(); err != nil {
			log.Fatalf("driver error: %v", err)
		}
	}()
	<-ctx.Done()
	driver.Stop()
}

func preflightChecks(config *config.DriverConfig) error {
	if config.MountBinaryS3 != "" {
		if _, err := os.Stat(config.MountBinaryS3); os.IsNotExist(err) {
			return fmt.Errorf("s3 mount binary not found in $PATH at %s: %v", config.MountBinaryS3, err)
		}
	} else {
		if _, err := exec.LookPath("mount-s3"); os.IsNotExist(err) {
			return fmt.Errorf("mount-s3 binary not found in $PATH: %v", err)
		}
	}
	if config.MountBinary != "" {
		if _, err := os.Stat(config.MountBinary); os.IsNotExist(err) {
			return fmt.Errorf("mount binary not found in $PATH at %s: %v", config.MountBinary, err)
		}
	} else {
		if _, err := exec.LookPath("mount"); os.IsNotExist(err) {
			return fmt.Errorf("mount binary not found in $PATH: %v", err)
		}
	}
	return nil
}
