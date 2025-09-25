-include .env
export

all: tidy fmt lint coverage integration-test

build: tidy fmt lint
	go build -a -ldflags="-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME)" -installsuffix cgo -o di-matrix-cli ./cmd

tidy:
	go mod tidy

fmt:
	go fmt ./...

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
