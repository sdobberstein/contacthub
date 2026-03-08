.PHONY: build test lint vuln run docker-build docker-up docker-down clean

BINARY  := bin/contacthub
VERSION := 0.1.0-dev
LDFLAGS := -ldflags="-w -s -X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/contacthub

test:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	go vet ./...
	golangci-lint run

vuln:
	govulncheck ./...
	gosec ./...

run:
	go run ./cmd/contacthub

docker-build:
	docker build -t contacthub:latest .

docker-up:
	docker compose up -d

docker-down:
	docker compose down

clean:
	rm -rf bin/ coverage.out coverage.html
