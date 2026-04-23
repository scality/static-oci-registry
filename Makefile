IMAGE_NAME ?= static-oci-registry
IMAGE_TAG ?= latest

IMAGES := \
  'my-solution v1.0.0 docker.io/library/alpine 3.19' \
  'my-solution v2.0.0 docker.io/library/alpine 3.20' \
  'my-solution v3.0.0-rc1 docker.io/library/alpine 3.21' \
  'my-solution v4.5.0 docker.io/library/alpine 3.22'

DOCKER_HOST ?= unix:///var/run/docker.sock
SKOPEO ?= skopeo
SKOPEO_FLAGS ?= --format v2s2 --dest-compress --src-daemon-host $(DOCKER_HOST) --insecure-policy

TEST_DOCKERFILE ?= ./test/target.Dockerfile
TEST_DIR ?= _testfs

.PHONY: build
build:
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

.PHONY: build-binary
build-binary:
	go build -o _build/static-oci-registry ./cmd/main.go

.PHONY: testfs
testfs: clean
	mkdir -p $(TEST_DIR)
	for img in $(IMAGES); do \
		set -- $$img; \
		solution=$$1; \
		version=$$2; \
		image=$$3; \
		tag=$$4; \
		docker build -f ./test/target.Dockerfile --build-arg VERSION=$$tag -t test-image:$$tag .; \
		mkdir -p $(TEST_DIR)/$$solution/$$version/$$image; \
		$(SKOPEO) copy $(SKOPEO_FLAGS) docker-daemon:test-image:$$tag dir:$(TEST_DIR)/$$solution/$$version/$$image/$$tag; \
	done > /dev/null

.PHONY: clean
clean:
	rm -rf $(TEST_DIR)

JUNIT_REPORT_DIR ?= .

.PHONY: unit-test
unit-test:
	DOCKER_HOST=$(DOCKER_HOST) TARGET_DOCKERFILE=$(realpath $(TEST_DOCKERFILE)) ginkgo --junit-report=$(JUNIT_REPORT_DIR)/junit.xml test/unit

.PHONY: integration-test
integration-test:
	DOCKER_HOST=$(DOCKER_HOST) TARGET_DOCKERFILE=$(realpath $(TEST_DOCKERFILE)) ginkgo --junit-report=$(JUNIT_REPORT_DIR)/junit.xml test/integration
