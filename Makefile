.PHONY: run build tidy fmt vet lint install-deps test docker-up docker-down docker-restart docker-logs docker-ps

BINARY := build/app/raccounting

BIN_DIR := bin
GOLANGCI_LINT := $(BIN_DIR)/golangci-lint

help: ## Show list of commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-10s\033[0m %s\n", $$1, $$2}'

build: ## Build binary for Linux (amd64) into build/app/raccounting
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(BINARY) ./cmd/raccounting

run: ## Run the application (go run ./cmd/raccounting)
	go run ./cmd/raccounting

tidy: ## Tidy up dependencies
	go mod tidy

fmt: ## Format code
	go fmt ./...

vet: ## Static analysis
	go vet ./...

install-deps: ## Fetch the latest golangci-lint into ./bin (re-run any time to upgrade it)
	GOBIN=$(CURDIR)/$(BIN_DIR) go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

test: ## Run tests
	go test ./...

lint: $(GOLANGCI_LINT) ## Run golangci-lint from ./bin (fetched via install-deps on first use)
	$(GOLANGCI_LINT) run ./...

docker-up: ## Start docker (MariaDB) in background
	docker compose up -d

docker-down: ## Stop and remove docker containers
	docker compose down

docker-restart: docker-down docker-up ## Restart docker

docker-logs: ## Docker container logs
	docker compose logs -f

docker-ps: ## Container status
	docker compose ps
