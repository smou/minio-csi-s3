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
	"os/signal"
	"syscall"

	"github.com/smou/k8s-csi-s3/pkg/config"
	"github.com/smou/k8s-csi-s3/pkg/driver"
)

func init() {
	flag.Set("logtostderr", "true")
}

var (
	endpoint    = flag.String("endpoint", "unix://tmp/csi.sock", "CSI endpoint")
	nodeID      = flag.String("nodeid", "controller", "kubernetes node id")
	mountBinary = flag.String("mountBinary", "mount", "s3 mount binary path")
)

func main() {
	flag.Parse()

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
	if config.MountBinary != "" {
		if _, err := os.Stat(config.MountBinary); os.IsNotExist(err) {
			return fmt.Errorf("mount binary not found at %s: %v", config.MountBinary, err)
		}
	} else {
		if _, err := os.Stat("mount"); os.IsNotExist(err) {
			return fmt.Errorf("mount binary not found: %v", err)
		}
	}
	if _, err := os.Stat("mountpoint-s3"); os.IsNotExist(err) {
		return fmt.Errorf("mountpoint-s3 binary not found: %v", err)
	}
	return nil
}
