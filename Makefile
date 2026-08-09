# Make targets for building and checking llmm.

GO ?= go

.PHONY: fmt test vet build cover

fmt:
	$(GO) fmt ./cmd ./internal

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

build:
	$(GO) build -o llmm ./cmd/llmm

cover:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out
