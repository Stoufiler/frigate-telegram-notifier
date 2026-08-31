VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
BINARY  := frigate-bot
PKG     := ./bot

.PHONY: build lint fmt test changelog clean docker

build:
	cd $(PKG) && go build -ldflags="-s -w -X main.version=$(VERSION)" -o $(BINARY) .

lint:
	cd $(PKG) && golangci-lint run ./...

fmt:
	cd $(PKG) && golangci-lint fmt ./...

test:
	cd $(PKG) && go test -race ./...

changelog:
	git cliff -o CHANGELOG.md

docker:
	docker build --build-arg VERSION=$(VERSION) -t $(BINARY):$(VERSION) -f $(PKG)/Dockerfile .

clean:
	rm -f $(PKG)/$(BINARY)
