#!/usr/bin/env bash
# Installs podman and configures it to trust the registry's CA.
# Inputs (env vars):
#   CA_CERT       — path to the CA certificate to trust (required)
#   REGISTRY_HOST — host:port of the registry (default: localhost:5000)
set -euxo pipefail

: "${CA_CERT:?CA_CERT must be set to the path of the CA certificate}"
REGISTRY_HOST="${REGISTRY_HOST:-localhost:5000}"

sudo apt-get update
sudo apt-get install -y --no-install-recommends podman

cert_dir="${HOME}/.config/containers/certs.d/${REGISTRY_HOST}"
mkdir -p "${cert_dir}"
cp "${CA_CERT}" "${cert_dir}/ca.crt"

podman info >/dev/null
