# Copyright (c) 2019 StackRox Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Makefile for building the Admission Controller webhook demo server + docker image.

.DEFAULT_GOAL := image

IMAGE ?= quay.io/maxgio92/admission-controller-webhook-demo
TAG ?= latest

.PHONY: image
image: image/build

.PHONY: image/build
image/build:
	@docker build . -t $(IMAGE):$(TAG)

.PHONY: image/push
image/push:
	docker push $(IMAGE):$(TAG)

.PHONY: image/dlv/build
image/dlv/build:
	docker build . --build-arg "GCFLAGS=all=-N -l" --tag $(IMAGE):dlv --target dlv

.PHONY: image/dlv/push
image/dlv/push: TAG := dlv
image/dlv/push: image/push

.PHONY: cluster
cluster:
	@kind create cluster --wait=30s || true

.PHONY: debug
debug: cluster
	@./hack/deploy.sh "deployment/deployment.debug.yaml.template"

.PHONY: deploy
deploy: cluster
	@./hack/deploy.sh
