SHELL=/bin/bash -o pipefail

BIN_DIR?=$(shell pwd)/tmp/bin

EMBEDMD_BIN=$(BIN_DIR)/embedmd
GOVVV_BIN=$(BIN_DIR)/govvv
GOTESTSUM_BIN=$(BIN_DIR)/gotestsum

TOOLING=$(GOVVV_BIN) $(GOTESTSUM_BIN)

.PHONY: dl
dl:
	@echo Download go.mod dependencies
	@go mod download

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

# https://marcofranssen.nl/manage-go-tools-via-go-modules/
# https://github.com/prometheus-operator/kube-prometheus/blob/master/Makefile
$(TOOLING): $(BIN_DIR)
	@echo Installing tools from hack/tools.go
	@cd hack && cat tools.go | grep _ | awk -F'"' '{print $$2}' | xargs -tI % go build -modfile=go.mod -o $(BIN_DIR) %

.PHONY: gen
gen: $(EASYJSON_BIN)
	go generate ./...

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: build
build: $(GOVVV_BIN)
	mkdir -p ./dist
	$(GOVVV_BIN) build -o ./dist/terraform-provider-sealedsecrets_v0.0.1 .
	chmod +x ./dist/terraform-provider-sealedsecrets_v0.0.1

.PHONY: test
test: $(GOTESTSUM_BIN)
	$(GOTESTSUM_BIN) --format testname ./... -- -count=1

.PHONY: cover
cover:
	go test -race -covermode atomic -coverprofile coverage.out ./...

install: build
	mkdir -p $$HOME/.terraform.d/plugins/tunein.com/tunein-incubator/sealedsecrets/0.0.1/darwin_amd64/
	cp ./dist/terraform-provider-sealedsecrets_v0.0.1 $$HOME/.terraform.d/plugins/tunein.com/tunein-incubator/sealedsecrets/0.0.1/darwin_amd64/

uninstall:
	rm $$HOME/.terraform.d/plugins/terraform-provider-sealedsecrets

# example: export TF_LOG = TRACE
example: install
	terraform init
	terraform apply -parallelism=1
	terraform output manifest

# PRE-COMMIT & GITHOOKS
# ---------------------
pre-commit.install:
	pre-commit install --install-hooks

pre-commit.run:
	pre-commit run --all-files

release: clean
	@echo "--skip-publish, as we will use github actions to do this"
	git-chglog -o CHANGELOG.md
	goreleaser --skip-publish --rm-dist
