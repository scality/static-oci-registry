FROM gcr.io/distroless/static-debian12@sha256:d75cdd72874d4790092fcb1b058493ecf6bb5bf2b2b897045b00ff01d91843f2

ARG VERSION=latest

LABEL maintainer="ayoub.nasr@scality.com"
LABEL org.opencontainers.image.title="Static OCI Registry test Image"
LABEL org.opencontainers.image.description="Minimal image used for testing Static OCI Registry functionality"
LABEL org.opencontainers.image.version="${VERSION}"
