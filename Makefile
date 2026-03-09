.PHONY: build test lint vuln run dev seed davlint install-tools docker-build docker-up docker-down clean setup

BINARY  := bin/contacthub
VERSION := 0.1.0-dev
LDFLAGS := -ldflags="-w -s -X main.version=$(VERSION)"

# Override to use a local build during davlint development:
#   make davlint DAVLINT="go run github.com/sdobberstein/davlint/cmd/davlint"
DAVLINT ?= davlint

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

dev:
	CONTACTHUB_DATABASE_PATH=./dev.db go run ./cmd/contacthub

# Seed the dev database with a test user and app password.
# Prints the chub_... token — paste it into davlint.yaml as the principal password.
seed:
	go run ./cmd/seed

# Run davlint conformance tests against the local dev server.
# Prerequisites:
#   1. make install-tools       (first time only)
#   2. cp davlint.example.yaml davlint.yaml
#   3. make seed                (paste printed token into davlint.yaml)
#   4. make dev                 (in another terminal)
#   5. make davlint
davlint:
	$(DAVLINT) run --config davlint.yaml

# Install all development tools. Re-run to upgrade.
install-tools:
	GOPROXY=direct go install github.com/sdobberstein/davlint/cmd/davlint@latest
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.2
	go install github.com/securego/gosec/v2/cmd/gosec@v2.24.7
	go install golang.org/x/vuln/cmd/govulncheck@v1.1.4

docker-build:
	docker build -t contacthub:latest .

docker-up:
	docker compose up -d

docker-down:
	docker compose down

setup:
	git config core.hooksPath hooks

clean:
	rm -rf bin/ coverage.out coverage.html dev.db
