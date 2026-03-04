BINARY    := kms-gateway
GOARCH    ?= arm64
GOOS      ?= linux

.PHONY: build build-local lint test docker clean

build:
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags="-s -w" \
		-o bin/$(BINARY)-$(GOOS)-$(GOARCH) ./cmd/kms-gateway/

build-local:
	go build -o bin/$(BINARY) ./cmd/kms-gateway/

test:
	go test -race -v ./...

lint:
	golangci-lint run

docker:
	docker buildx build --platform linux/arm64 -t $(BINARY):latest .

clean:
	rm -rf bin/
