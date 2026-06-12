IMAGE_NAME ?= static-oci-registry
IMAGE_TAG ?= latest

SHELL := /bin/bash

IMAGES := \
  'my-solution v1.0.0 docker.io/library/alpine 3.19' \
  'my-solution v2.0.0 docker.io/library/alpine 3.20' \
  'my-solution v3.0.0-rc1 docker.io/library/alpine 3.21' \
  'my-solution v4.5.0 docker.io/library/alpine 3.22'

DOCKER_HOST ?= unix:///var/run/docker.sock
SKOPEO ?= skopeo
SKOPEO_FLAGS ?= --format v2s2 --dest-compress --src-daemon-host $(DOCKER_HOST) --insecure-policy

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
		docker build -f ./test/target.Dockerfile --build-arg VERSION=$$tag -t test-image:$$tag .; \
		mkdir -p $(TEST_DIR)/$$solution/$$version/$$image; \
		$(SKOPEO) copy $(SKOPEO_FLAGS) docker-daemon:test-image:$$tag dir:$(TEST_DIR)/$$solution/$$version/$$image/$$tag; \
	done > /dev/null
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

.PHONY: unit-test
unit-test:
	DOCKER_HOST=$(DOCKER_HOST) TARGET_DOCKERFILE=$(realpath $(TEST_DOCKERFILE)) ginkgo --junit-report=$(JUNIT_REPORT_DIR)/junit-unit.xml test/unit

.PHONY: integration-test
integration-test:
	DOCKER_HOST=$(DOCKER_HOST) TARGET_DOCKERFILE=$(realpath $(TEST_DOCKERFILE)) ginkgo --junit-report=$(JUNIT_REPORT_DIR)/junit-integration.xml test/integration

.PHONY: e2e-test
e2e-test:
	ginkgo --junit-report=$(JUNIT_REPORT_DIR)/junit-e2e.xml test/e2e
