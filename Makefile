# build: build docker image and tag it as static-oci-registy:latest
build:
	docker build -t static-oci-registry:latest .
.PHONY: build

IMAGES := \
  'my-solution 1.0 docker.io/library/postgres 9' \
  'my-solution 2.0 docker.io/library/postgres 13' \
  'my-solution 2.5 docker.io/library/postgres 16' \
  'my-solution 3.0 docker.io/library/postgres 17'

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
