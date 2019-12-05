.PHONY: build generate push-only push

IMAGE_REGISTRY:=us.gcr.io
IMAGE_TAG:=latest

IMAGE:=planetscale-vitess/operator

IMAGE_NAME:=$(IMAGE_REGISTRY)/$(IMAGE)

OPERATOR_SDK_VERSION:=v0.10.0

# Enable Go modules
export GO111MODULE=on

build:
	operator-sdk-$(OPERATOR_SDK_VERSION) build $(IMAGE_NAME):$(IMAGE_TAG) --image-build-args '--no-cache'

generate:
	operator-sdk-$(OPERATOR_SDK_VERSION) generate k8s
	operator-sdk-$(OPERATOR_SDK_VERSION) generate openapi
	go run github.com/ahmetb/gen-crd-api-reference-docs -api-dir ./pkg/apis -config docs/api/config.json -template-dir docs/api/template -out-file docs/api/index.html

push-only: DATE=$(shell date -I)
push-only: GITHASH=$(shell git rev-parse HEAD)
push-only:
	docker tag $(IMAGE_NAME):$(IMAGE_TAG) $(IMAGE_NAME):$(DATE)-$(GITHASH)
	docker push $(IMAGE_NAME):$(IMAGE_TAG)
	docker push $(IMAGE_NAME):$(DATE)-$(GITHASH)

push: build
push: push-only
