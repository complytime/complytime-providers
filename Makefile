BINARY_DIR ?= bin

.PHONY: build build-openscap-provider build-ampel-provider test vendor lint

build: build-openscap-provider build-ampel-provider

build-openscap-provider:
	go build -o $(BINARY_DIR)/complyctl-provider-openscap ./cmd/openscap-provider

build-ampel-provider:
	go build -o $(BINARY_DIR)/complyctl-provider-ampel ./cmd/ampel-provider

test:
	go test ./...

vendor:
	go mod vendor

lint:
	golangci-lint run ./...
