# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

SHELL = /bin/bash
VERSION=1.1.0
NAME=minio.csi.s3
GIT_COMMIT?=$(shell git rev-parse HEAD)
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
PKG=github.com/smou/k8s-csi-s3

LDFLAGS?="-w -s -X ${PKG}/pkg/driver/version.driverVersion=${VERSION} \
-X ${PKG}/pkg/driver/version.driverName=${NAME} \
-X ${PKG}/pkg/driver/version.gitCommit=${GIT_COMMIT} \
-X ${PKG}/pkg/driver/version.buildDate=${BUILD_DATE}"

GOOS=$(shell go env GOOS)
CGO_ENABLED=0
GOFLAGS := -a
GOENV := CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=amd64

BINARY_NAME := s3driver
SRC_DIR := $(shell pwd)/cmd/s3driver
PKG_DIR := $(shell pwd)/pkg
OUTPUT_DIR := $(shell pwd)/_output

.PHONY: sync test build clean
all: sync test build
sync:
	@echo "==> Syncing dependencies"
	go mod tidy
test:
	@echo "==> Running unit tests"
	go test ${PKG_DIR}/... ${GOFLAGS} -v -race
build: clean
	@echo "==> Building binary"
	mkdir -p $(OUTPUT_DIR)
	${GOENV} go build ${GOFLAGS} -ldflags ${LDFLAGS} -o ${OUTPUT_DIR}/${BINARY_NAME} ${SRC_DIR}
clean:
	@echo "==> Cleaning"
	go clean -r -x
	rm -rf $(OUTPUT_DIR)/*
