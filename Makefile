.PHONY: test test-integration test-e2e test-all lint vet generate build coverage coverage-integration coverage-html coverage-html-integration deadcode

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

coverage:
	go test -race -coverprofile=coverage.unit.out -covermode=atomic ./...
	go tool cover -func=coverage.unit.out

coverage-integration:
	go test -race -tags=integration -coverprofile=coverage.integration.out -covermode=atomic ./...
	go tool cover -func=coverage.integration.out

coverage-html: coverage
	go tool cover -html=coverage.unit.out -o coverage.html
	@echo "Report: coverage.html"

coverage-html-integration: coverage-integration
	go tool cover -html=coverage.integration.out -o coverage.integration.html
	@echo "Report: coverage.integration.html"

deadcode:
	go run golang.org/x/tools/cmd/deadcode@latest -test ./...
