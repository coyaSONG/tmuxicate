BINARY := tmuxicate
MODULE := github.com/coyaSONG/tmuxicate
GOFLAGS := -trimpath

.PHONY: build test test-integration lint fmt ci install clean

build:
	go build $(GOFLAGS) -o bin/$(BINARY) ./cmd/tmuxicate

test:
	go test ./... -count=1 -race

test-integration:
	go test ./... -count=1 -race -tags=integration

lint:
	golangci-lint run ./...

fmt:
	gofumpt -w .
	goimports -w .

ci: lint test

install:
	go install $(GOFLAGS) ./cmd/tmuxicate

clean:
	rm -rf bin/ dist/
