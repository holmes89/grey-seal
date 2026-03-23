.PHONY: test test-integration test-e2e test-all lint vet generate build

test:
	go test -race ./...

test-integration:
	go test -race -tags=integration ./...

test-e2e:
	go test -race -tags=e2e ./...

test-all:
	go test -race -tags="integration e2e" ./...

lint:
	golangci-lint run ./...

vet:
	go vet ./...

generate:
	go run github.com/vektra/mockery/v2@latest --config .mockery.yaml

build:
	go build -o bin/api ./cmd/api
