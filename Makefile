# Copyright 2019 The Volcano Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

BIN_DIR=_output/bin
RELEASE_DIR=_output/release
REPO_PATH=pkg.yezhisheng.me/volcano
IMAGE_PREFIX=volcanosh/vc
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true,allowDangerousTypes=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Get OS architecture
OSARCH=$(shell uname -m)
ifeq ($(OSARCH),x86_64)
REL_OSARCH=linux/amd64
else ifeq ($(OSARCH),x64)
REL_OSARCH=linux/amd64
else ifeq ($(OSARCH),aarch64)
REL_OSARCH=linux/arm64
else ifeq ($(OSARCH),aarch64_be)
REL_OSARCH=linux/arm64
else ifeq ($(OSARCH),armv8b)
REL_OSARCH=linux/arm64
else ifeq ($(OSARCH),armv8l)
REL_OSARCH=linux/arm64
else ifeq ($(OSARCH),i386)
REL_OSARCH=linux/x86
else ifeq ($(OSARCH),i686)
REL_OSARCH=linux/x86
else ifeq ($(OSARCH),arm)
REL_OSARCH=linux/arm
else
REL_OSARCH=linux/$(OSARCH)
endif

include Makefile.def

.EXPORT_ALL_VARIABLES:

all: vc-scheduler vc-controller-manager vc-webhook-manager vcctl command-lines

init:
	mkdir -p ${BIN_DIR}
	mkdir -p ${RELEASE_DIR}

vc-scheduler: init
	go build -ldflags ${LD_FLAGS} -o=${BIN_DIR}/vc-scheduler ./cmd/scheduler

vc-controller-manager: init
	go build -ldflags ${LD_FLAGS} -o=${BIN_DIR}/vc-controller-manager ./cmd/controller-manager

vc-webhook-manager: init
	go build -ldflags ${LD_FLAGS} -o=${BIN_DIR}/vc-webhook-manager ./cmd/webhook-manager

vcctl: init
	go build -ldflags ${LD_FLAGS} -o=${BIN_DIR}/vcctl ./cmd/cli

image_bins: init
	GO111MODULE=off go get github.com/mitchellh/gox
	CGO_ENABLED=0 gox -osarch=${REL_OSARCH} -ldflags ${LD_FLAGS} -output ${BIN_DIR}/${REL_OSARCH}/vcctl ./cmd/cli
	for name in controller-manager scheduler webhook-manager; do\
		CGO_ENABLED=0 gox -osarch=${REL_OSARCH} -ldflags ${LD_FLAGS} -output ${BIN_DIR}/${REL_OSARCH}/vc-$$name ./cmd/$$name; \
	done

images: image_bins
	for name in controller-manager scheduler webhook-manager; do\
		cp ${BIN_DIR}/${REL_OSARCH}/vc-$$name ./installer/dockerfile/$$name/;\
		if [ ${REL_OSARCH} = linux/amd64 ];then\
			docker build --no-cache -t $(IMAGE_PREFIX)-$$name:$(TAG) ./installer/dockerfile/$$name;\
		elif [ ${REL_OSARCH} = linux/arm64 ];then\
			docker build --no-cache -t $(IMAGE_PREFIX)-$$name-arm64:$(TAG) -f ./installer/dockerfile/$$name/Dockerfile.arm64 ./installer/dockerfile/$$name;\
		else\
			echo "only support x86_64 and arm64. Please build image according to your architecture";\
		fi;\
		rm installer/dockerfile/$$name/vc-$$name;\
	done

webhook-manager-base-image:
	if [ ${REL_OSARCH} = linux/amd64 ];then\
		docker build --no-cache -t $(IMAGE_PREFIX)-webhook-manager-base:$(TAG) ./installer/dockerfile/webhook-manager/ -f ./installer/dockerfile/webhook-manager/Dockerfile.base;\
	elif [ ${REL_OSARCH} = linux/arm64 ];then\
		docker build --no-cache -t $(IMAGE_PREFIX)-webhook-manager-base-arm64:$(TAG) ./installer/dockerfile/webhook-manager/ -f ./installer/dockerfile/webhook-manager/Dockerfile.base.arm64;\
	else\
		echo "only support x86_64 and arm64. Please build webhook-manager-base-image according to your architecture";\
	fi

generate-code:
	./hack/update-gencode.sh

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./pkg/apis/scheduling/v1beta1;./pkg/apis/batch/v1alpha1;./pkg/apis/bus/v1alpha1;" output:crd:artifacts:config=config/crd/bases

unit-test:
	go clean -testcache
	go list ./... | grep -v e2e | xargs go test -p 8 -v -race

e2e:
	./hack/run-e2e-kind.sh

e2e-test-schedulingbase:
	E2E_TYPE=SCHEDULINGBASE ./hack/run-e2e-kind.sh

e2e-test-schedulingaction:
	E2E_TYPE=SCHEDULINGACTION ./hack/run-e2e-kind.sh

e2e-test-jobp:
	E2E_TYPE=JOBP ./hack/run-e2e-kind.sh

e2e-test-jobseq:
	E2E_TYPE=JOBSEQ ./hack/run-e2e-kind.sh

generate-yaml: init
	./hack/generate-yaml.sh TAG=${RELEASE_VER}

release-env:
	./hack/build-env.sh release

dev-env:
	./hack/build-env.sh dev

release: images generate-yaml
	./hack/publish.sh

clean:
	rm -rf _output/
	rm -f *.log

verify:
	hack/verify-gofmt.sh
	hack/verify-golint.sh
	hack/verify-gencode.sh
	hack/verify-vendor.sh
	hack/verify-vendor-licenses.sh

lint: ## Lint the files
	golangci-lint version
	golangci-lint run pkg/kube pkg/version pkg/apis/...

verify-generated-yaml:
	./hack/check-generated-yaml.sh

command-lines:
	go build -ldflags ${LD_FLAGS} -o=${BIN_DIR}/vcancel ./cmd/cli/vcancel
	go build -ldflags ${LD_FLAGS} -o=${BIN_DIR}/vresume ./cmd/cli/vresume
	go build -ldflags ${LD_FLAGS} -o=${BIN_DIR}/vsuspend ./cmd/cli/vsuspend
	go build -ldflags ${LD_FLAGS} -o=${BIN_DIR}/vjobs ./cmd/cli/vjobs
	go build -ldflags ${LD_FLAGS} -o=${BIN_DIR}/vqueues ./cmd/cli/vqueues
	go build -ldflags ${LD_FLAGS} -o=${BIN_DIR}/vsub ./cmd/cli/vsub

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif
