#!/usr/bin/env bash
# Configures the system containerd to:
#   - run the CRI plugin (disabled by default in the docker-shipped containerd
#     on ubuntu-latest),
#   - trust the registry's CA via /etc/containerd/certs.d,
# then installs crictl and writes /etc/crictl.yaml so plain `crictl ...`
# invocations (no env vars, no flags) work for any user.
#
# The containerd socket is chmod 666 so the unprivileged runner user can talk
# to it without sudo. This is CI-only; never do this on a shared host.
#
# Inputs (env vars):
#   CA_CERT        — path to the CA certificate to trust (required)
#   REGISTRY_HOST  — host:port of the registry (default: localhost:5000)
#   CRICTL_VERSION — crictl release to install   (default: v1.31.1)
set -euxo pipefail

: "${CA_CERT:?CA_CERT must be set to the path of the CA certificate}"
REGISTRY_HOST="${REGISTRY_HOST:-localhost:5000}"
CRICTL_VERSION="${CRICTL_VERSION:-v1.31.1}"

# --- 1. Configure containerd ------------------------------------------------

certs_d="/etc/containerd/certs.d/${REGISTRY_HOST}"
sudo mkdir -p /etc/containerd "${certs_d}"
sudo cp "${CA_CERT}" "${certs_d}/ca.crt"

sudo tee "${certs_d}/hosts.toml" >/dev/null <<EOF
[host."https://${REGISTRY_HOST}"]
  capabilities = ["pull", "resolve"]
  ca = "ca.crt"
EOF

sudo tee /etc/containerd/config.toml >/dev/null <<'EOF'
version = 2

[plugins."io.containerd.grpc.v1.cri".registry]
  config_path = "/etc/containerd/certs.d"

[grpc]
  address = "/run/containerd/containerd.sock"
EOF

sudo systemctl restart containerd

# Wait for the socket to come back after restart.
for _ in $(seq 1 20); do
  [ -S /run/containerd/containerd.sock ] && break
  sleep 0.5
done
[ -S /run/containerd/containerd.sock ]

# Open the socket to the runner user.
sudo chmod 666 /run/containerd/containerd.sock

# --- 2. Install crictl ------------------------------------------------------

tarball="/tmp/crictl-${CRICTL_VERSION}.tar.gz"
curl -fsSL -o "${tarball}" \
  "https://github.com/kubernetes-sigs/cri-tools/releases/download/${CRICTL_VERSION}/crictl-${CRICTL_VERSION}-linux-amd64.tar.gz"
sudo tar -C /usr/local/bin -xzf "${tarball}"
rm -f "${tarball}"

sudo tee /etc/crictl.yaml >/dev/null <<'EOF'
runtime-endpoint: unix:///run/containerd/containerd.sock
image-endpoint: unix:///run/containerd/containerd.sock
timeout: 10
EOF

# --- 3. Smoke test ----------------------------------------------------------

crictl info >/dev/null
crictl images
