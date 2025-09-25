include .env
export

all: tidy fmt lint coverage integration-test

tidy:
	go mod tidy

fmt:
	golangci-lint fmt

lint: fmt
	golangci-lint run

test:
	go test -race ./...

coverage:
	gotestsum -- -coverprofile=cover.out ./...

integration-test:
	go test -tags=integration ./... --timeout=30s

run:
	go run cmd/main.go analyze --config config.yaml --language nodejs
