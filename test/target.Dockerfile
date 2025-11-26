FROM gcr.io/distroless/static-debian12

ARG VERSION=latest

LABEL maintainer="ayoub.nasr@scality.com"
LABEL org.opencontainers.image.title="Static OCI Registry test Image"
LABEL org.opencontainers.image.description="Minimal image used for testing Static OCI Registry functionality"
LABEL org.opencontainers.image.version="${VERSION}"
