# build: build docker image and tag it as static-oci-registy:latest
build:
	docker build -t static-oci-registry:latest .
.PHONY: build

# TODO: add another solution here
IMAGES := \
  'my-solution v1.0.0 docker.io/library/postgres 9' \
  'my-solution v2.0.1-rc.1 docker.io/library/postgres 13' \
  'my-solution v2.0.1 docker.io/library/postgres 16' \
  'my-solution 2.5.8-beta.1 docker.io/library/postgres 17' \
  'my-solution 3.0.9-pw.1 docker.io/library/postgres 18' \
  'another-solution 1.0 docker.io/library/alpine 3.17' \
  'another-solution 2.0 docker.io/library/alpine 3.21' \
  'another-solution 2.7 docker.io/library/alpine 3.22'

testfs: clean
	mkdir -p _testfs
	for img in $(IMAGES); do \
		set -- $$img; \
		solution=$$1; \
		version=$$2; \
		image=$$3; \
		tag=$$4; \
		mkdir -p _testfs/$$solution/$$version/$$image; \
		skopeo copy docker://$$image:$$tag dir:_testfs/$$solution/$$version/$$image/$$tag; \
	done

clean:
	rm -rf _testfs
