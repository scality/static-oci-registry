IMAGE_NAME ?= static-oci-registry
IMAGE_TAG ?= latest

SHELL := /bin/bash

IMAGES := \
  'my-solution v1.0.0 docker.io/library/alpine 3.19' \
  'my-solution v2.0.0 docker.io/library/alpine 3.20' \
  'my-solution v3.0.0-rc1 docker.io/library/alpine 3.21' \
  'my-solution v4.5.0 docker.io/library/alpine 3.22'

SKOPEO ?= skopeo
SKOPEO_FLAGS ?= --all --insecure-policy

DOCKER_HOST ?= unix:///var/run/docker.sock
TEST_DOCKERFILE ?= ./test/target.Dockerfile

TEST_DIR ?= ./_testfs
CERT_DIR ?= ./_certs

.PHONY: build
build:
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

.PHONY: build-binary
build-binary:
	go build -o _build/static-oci-registry ./cmd/main.go

.PHONY: testfs
testfs: clean
	@mkdir -p $(TEST_DIR)
	@for img in $(IMAGES); do \
		set -- $$img; \
		solution=$$1; \
		version=$$2; \
		image=$$3; \
		tag=$$4; \
		mkdir -p $(TEST_DIR)/$$solution/$$version/$$image; \
		$(SKOPEO) copy $(SKOPEO_FLAGS) docker://$$image:$$tag oci:$(TEST_DIR)/$$solution/$$version/$$image:$$tag; \
	done
	@echo
	@echo "Created test filesystem in $(TEST_DIR), use:"
	@echo "FS_ROOT=$(TEST_DIR)"
	@echo

.PHONY: testcert
testcert:
	@mkdir -p $(CERT_DIR)
	@openssl genrsa -out $(CERT_DIR)/ca.key 4096
	@openssl req -new -x509 -days 3650 -key $(CERT_DIR)/ca.key \
		-subj "/CN=static-oci-registry-local-ca" \
		-out $(CERT_DIR)/ca.crt
	@openssl genrsa -out $(CERT_DIR)/server.key 4096
	@openssl req -new -key $(CERT_DIR)/server.key \
		-subj "/CN=localhost" \
		-out $(CERT_DIR)/server.csr
	@openssl x509 -req -days 3650 -in $(CERT_DIR)/server.csr \
		-CA $(CERT_DIR)/ca.crt -CAkey $(CERT_DIR)/ca.key -CAcreateserial \
		-extfile <(printf "subjectAltName=DNS:localhost,IP:127.0.0.1") \
		-out $(CERT_DIR)/server.crt
	@echo
	@echo "Generated test certificates in $(CERT_DIR), use:"
	@echo "HTTP_TLS_CERT_FILE=$(CERT_DIR)/server.crt"
	@echo "HTTP_TLS_KEY_FILE=$(CERT_DIR)/server.key"
	@echo

.PHONY: clean
clean:
	rm -rf $(TEST_DIR) $(CERT_DIR)

JUNIT_REPORT_DIR ?= .

## Tool Binaries
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p "$(LOCALBIN)"

ENVTEST ?= $(LOCALBIN)/setup-envtest

#ENVTEST_VERSION is the version of controller-runtime release branch to fetch the envtest setup script (i.e. release-0.20)
ENVTEST_VERSION ?= $(shell v='$(call gomodver,sigs.k8s.io/controller-runtime)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_VERSION manually (controller-runtime replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?([0-9]+)\.([0-9]+).*/release-\1.\2/')

#ENVTEST_K8S_VERSION is the version of Kubernetes to use for setting up ENVTEST binaries (i.e. 1.31)
ENVTEST_K8S_VERSION ?= $(shell v='$(call gomodver,k8s.io/api)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_K8S_VERSION manually (k8s.io/api replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?[0-9]+\.([0-9]+).*/1.\1/')

.PHONY: unit-test
unit-test:
	ginkgo --junit-report=$(JUNIT_REPORT_DIR)/junit-unit.xml test/unit
	go test ./pkg/...

.PHONY: integration-test
integration-test:
	DOCKER_HOST=$(DOCKER_HOST) TARGET_DOCKERFILE=$(realpath $(TEST_DOCKERFILE)) ginkgo --junit-report=$(JUNIT_REPORT_DIR)/junit-integration.xml test/integration

.PHONY: e2e-test
e2e-test:
	ginkgo --junit-report=$(JUNIT_REPORT_DIR)/junit-e2e.xml test/e2e

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: setup-envtest ## Download the binaries required for ENVTEST in the local bin directory.
setup-envtest: envtest
	@echo "Setting up envtest binaries for Kubernetes version $(ENVTEST_K8S_VERSION)..."
	@"$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path || { \
		echo "Error: Failed to set up envtest binaries for version $(ENVTEST_K8S_VERSION)."; \
		exit 1; \
	}

.PHONY: kube-test ## Run kube tests
kube-test: setup-envtest
	KUBEBUILDER_ASSETS="$(shell "$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path)" ginkgo --junit-report=$(JUNIT_REPORT_DIR)/junit-kube.xml test/kube

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] && [ "$$(readlink -- "$(1)" 2>/dev/null)" = "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f "$(1)" ;\
GOBIN="$(LOCALBIN)" go install $${package} ;\
mv "$(LOCALBIN)/$$(basename "$(1)")" "$(1)-$(3)" ;\
} ;\
ln -sf "$$(realpath "$(1)-$(3)")" "$(1)"
endef

define gomodver
$(shell go list -m -f '{{if .Replace}}{{.Replace.Version}}{{else}}{{.Version}}{{end}}' $(1) 2>/dev/null)
endef
