FROM alpine:3.23 AS mountpoint

RUN apk add --no-cache curl tar
ARG MOUNTPOINT_VERSION=1.22.0
ARG TARGETARCH=x86_64
RUN curl -L \
    https://s3.amazonaws.com/mountpoint-s3-release/${MOUNTPOINT_VERSION}/${TARGETARCH}/mount-s3-${MOUNTPOINT_VERSION}-${TARGETARCH}.tar.gz \
    | tar xz && \
    mv bin/mount-s3 /mount-s3 && \
    chmod 0755 /mount-s3

FROM alpine:3.23
LABEL maintainers="Christopher Schütze <https://github.com/smou>"
LABEL description="minio-csi-s3 slim image"

# Minimal runtime deps
RUN apk add --no-cache ca-certificates util-linux && \
    addgroup -S csi && \
    adduser -S -G csi -u 10001 csi

COPY --from=mountpoint /mount-s3 /usr/local/bin/mount-s3
COPY --from=mountpoint /mount-s3 /usr/local/bin/mountpoint-s3
COPY _output/s3driver /usr/local/bin/s3driver
# Permissions
RUN chmod 0755 /usr/local/bin/mountpoint-s3 /usr/local/bin/s3driver
USER 10001:10001
ENTRYPOINT ["/usr/local/bin/s3driver"]
