FROM golang:1.27.3-alpine@sha256:d200d2dd76b03a99b52651fd1325aff7dcd35422c35c01bac56571b0507f3deb AS builder

ARG APPLICATION_VERSION=dev

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY cmd/ cmd/
COPY pkg/ pkg/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build --ldflags "-X 'github.com/scality/static-oci-registry/cmd/config.ApplicationVersion=${APPLICATION_VERSION}'" -o static-oci-registry ./cmd/main.go

FROM scratch

LABEL org.opencontainers.image.source=https://github.com/scality/static-oci-registry

COPY --from=builder /app/static-oci-registry /bin/static-oci-registry

ENTRYPOINT ["/bin/static-oci-registry"]
