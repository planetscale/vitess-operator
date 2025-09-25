.PHONY: build release-build unit-test integration-test generate generate-and-diff generate-operator-yaml push-only push

IMAGE_REGISTRY:=docker.io
IMAGE_TAG:=latest

IMAGE:=planetscale/vitess-operator

IMAGE_NAME:=$(IMAGE_REGISTRY)/$(IMAGE)

# Enable Go modules
export GO111MODULE=on

build:
	CGO_ENABLED=0 go build -o build/_output/bin/vitess-operator ./cmd/manager

# Release build is slow but self-contained (doesn't depend on anything in your
# local machine). We use this for automated builds that we publish.
release-build:
	docker build -f build/Dockerfile.release -t $(IMAGE_NAME):$(IMAGE_TAG) .

release-build.arm64:
	docker buildx build --platform linux/arm64 --build-arg GOOS=linux --build-arg GOARCH=arm64 -f build/Dockerfile.release -t $(IMAGE_NAME):$(IMAGE_TAG) .

unit-test:
	pkgs="$$(go list ./... | grep -v '/test/integration/')" && \
		go test -count=1 $${pkgs}

integration-test:
	tools/get-kube-binaries.sh
	PATH="$(PWD)/tools/_bin:$(PATH)" go test -v -timeout 5m -count=1 ./test/integration/... -args --logtostderr -v=6

generate:
	go run sigs.k8s.io/controller-tools/cmd/controller-gen object crd:maxDescLen=0 paths="./pkg/apis/planetscale/v2" output:crd:artifacts:config=./deploy/crds
	go run github.com/ahmetb/gen-crd-api-reference-docs -api-dir planetscale.dev/vitess-operator/pkg/apis/planetscale/v2 -config ./docs/api/config.json -template-dir ./docs/api/template -out-file ./docs/api/index.html
	@echo 'Copying ./docs/api/index.html to ./docs/api.md'
	@cp ./docs/api/index.html ./docs/api.md && ex -sc '1,2d|x' ./docs/api.md
	@echo 'Written to ./docs/api.md'

generate-and-diff: generate
	git add --all
	git diff HEAD
	@echo 'If this test fails, it is because the git diff is non-empty after running "make generate".'
	@echo 'To correct this, locally run "make generate", commit the changes, and re-run tests.'
	git diff HEAD --quiet --exit-code

generate-operator-yaml:
	go run github.com/kubernetes-sigs/kustomize build ./deploy > build/_output/operator.yaml

push-only: DATE=$(shell date -I)
push-only: GITHASH=$(shell git rev-parse HEAD)
push-only:
	docker tag $(IMAGE_NAME):$(IMAGE_TAG) $(IMAGE_NAME):$(DATE)-$(GITHASH)
	docker push $(IMAGE_NAME):$(IMAGE_TAG)
	docker push $(IMAGE_NAME):$(DATE)-$(GITHASH)

push: build
push: push-only

# Setup for the end to end tests
e2e-test-setup:
	./tools/get-e2e-test-deps.sh

# Upgrade test
upgrade-test: e2e-test-setup
	echo "Running Upgrade test"
	test/endtoend/upgrade_test.sh

backup-restore-test: e2e-test-setup
	echo "Running Backup-Restore test"
	test/endtoend/backup_restore_test.sh

backup-schedule-test: e2e-test-setup
	echo "Running Backup-Schedule test"
	test/endtoend/backup_schedule_test.sh

vtorc-vtadmin-test: e2e-test-setup
	echo "Running VTOrc and VtAdmin test"
	test/endtoend/vtorc_vtadmin_test.sh

unmanaged-tablet-test: e2e-test-setup
	echo "Running Unmanaged Tablet test"
	test/endtoend/unmanaged_tablet_test.sh

hpa-test: e2e-test-setup
	echo "Running HPA test"
	test/endtoend/hpa_test.sh
