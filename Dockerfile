FROM golang:1.25.1-alpine AS builder

ARG APPLICATION_VERSION=dev

WORKDIR /app

COPY cmd/ cmd/
COPY pkg/ pkg/
COPY go.mod go.sum ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build --ldflags "-X 'github.com/scality/static-oci-registry/cmd/config.ApplicationVersion=${APPLICATION_VERSION}'" -o static-oci-registry ./cmd/main.go

FROM scratch

LABEL org.opencontainers.image.source=https://github.com/scality/static-oci-registry

COPY --from=builder /app/static-oci-registry /rootfs/usr/local/lib/containers/static-oci-registry/static-oci-registry

ENTRYPOINT ["/rootfs/usr/local/lib/containers/static-oci-registry/static-oci-registry"]
