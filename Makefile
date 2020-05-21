VERSION ?= v0.0.1
GOPROXY ?= https://goproxy.io,direct
DOCKER_REG ?= supremind

OPERATOR_IMAGE = $(DOCKER_REG)/container-snapshot-operator:$(VERSION)
WORKER_IMAGE = $(DOCKER_REG)/container-snapshot-worker:$(VERSION)

operator:
	DOCKER_BUILDKIT=1 docker build \
	--build-arg GOPROXY=$(GOPROXY) \
	--target release-operator \
	-t $(OPERATOR_IMAGE) \
	-f build/Dockerfile .

worker:
	DOCKER_BUILDKIT=1 docker build \
	--build-arg GOPROXY=$(GOPROXY) \
	--target release-worker \
	-t $(WORKER_IMAGE) \
	-f build/Dockerfile .

images: operator worker


push-operator: operator
	docker push $(OPERATOR_IMAGE)

push-worker: worker
	docker push $(WORKER_IMAGE)

push: push-operator push-worker


all: push

.PHONY: images 
