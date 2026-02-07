ARG $TARGETPLATFORM=linux/amd64

FROM --platform=$TARGETPLATFORM public.ecr.aws/amazonlinux/amazonlinux:2 AS mountpoint
ARG MOUNTPOINT_VERSION=1.22.0
ARG TARGETARCH=x86_64
RUN yum install -y wget gzip tar fuse-libs binutil patchelf
RUN wget -q https://s3.amazonaws.com/mountpoint-s3-release/${MOUNTPOINT_VERSION}/${TARGETARCH}/mount-s3-${MOUNTPOINT_VERSION}-${TARGETARCH}.tar.gz && \
    mkdir -p /mountpoint-s3 && \
    tar -xvzf mount-s3-${MOUNTPOINT_VERSION}-${TARGETARCH}.tar.gz -C /mountpoint-s3

FROM --platform=$BUILDPLATFORM public.ecr.aws/eks-distro-build-tooling/golang:1.25.5 as builder
ARG TARGETARCH
WORKDIR /minio-csi-s3-driver
COPY . .
RUN make

FROM --platform=$TARGETPLATFORM public.ecr.aws/eks-distro-build-tooling/eks-distro-minimal-base-csi:latest
LABEL maintainers="Christopher Schütze <https://github.com/smou>"
LABEL description="minio-csi-s3 slim image"
COPY --from=builder /minio-csi-s3-driver/_output/s3driver /usr/local/bin/
COPY --from=mountpoint /mountpoint-s3/bin/mount-s3 /usr/local/bin/
COPY --from=mountpoint /lib64/libfuse.so.2 /lib64/libfuse.so.2
RUN mkdir -p /run/csi && \
    chown 10001 /run/csi && \
    chmod 0775 /run/csi && \
    chmod 0755 /usr/local/bin/mount-s3 /usr/local/bin/s3driver
USER 10001
ENTRYPOINT ["/usr/local/bin/s3driver"]
