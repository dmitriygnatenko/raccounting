.PHONY: run build tidy fmt vet lint install-deps test css css-watch docker-up docker-down docker-restart docker-logs docker-ps

BINARY := build/app/raccounting

BIN_DIR := bin
GOLANGCI_LINT := $(BIN_DIR)/golangci-lint

TAILWIND_VERSION := 3.4.17
TAILWIND := $(BIN_DIR)/tailwindcss
TAILWIND_OS := $(shell uname -s | sed 's/Darwin/macos/;s/Linux/linux/')
TAILWIND_ARCH := $(shell uname -m | sed 's/x86_64/x64/;s/aarch64/arm64/')
TAILWIND_IN := web/tailwind.input.css
TAILWIND_OUT := web/css/tailwind.css

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

$(TAILWIND):
	mkdir -p $(BIN_DIR)
	curl -fsSL -o $@ https://github.com/tailwindlabs/tailwindcss/releases/download/v$(TAILWIND_VERSION)/tailwindcss-$(TAILWIND_OS)-$(TAILWIND_ARCH)
	chmod +x $@

css: $(TAILWIND) ## Build web/css/tailwind.css (re-run after changing classes in web/)
	$(TAILWIND) -c tailwind.config.js -i $(TAILWIND_IN) -o $(TAILWIND_OUT) --minify

css-watch: $(TAILWIND) ## Rebuild web/css/tailwind.css on every change in web/
	$(TAILWIND) -c tailwind.config.js -i $(TAILWIND_IN) -o $(TAILWIND_OUT) --watch

docker-up: ## Start docker (MariaDB) in background
	docker compose up -d

docker-down: ## Stop and remove docker containers
	docker compose down

docker-restart: docker-down docker-up ## Restart docker

docker-logs: ## Docker container logs
	docker compose logs -f

docker-ps: ## Container status
	docker compose ps
