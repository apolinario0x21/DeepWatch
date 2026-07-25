BINARY      ?= deepwatch
CMD_PATH    ?= ./cmd/deepwatch
IMAGE       ?= deepwatch:latest
COVER_FILE  ?= coverage.out
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS     := -s -w -X main.version=$(VERSION)

.PHONY: help build run test cover lint fmt vet tidy docker-build up down clean

help: ## Lista os targets disponíveis
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

build: ## Compila o binário estático em ./bin
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(BINARY) $(CMD_PATH)

run: ## Executa a aplicação localmente
	go run $(CMD_PATH)

test: ## Roda os testes com race detector
	go test ./... -race -count=1

cover: ## Gera relatório de cobertura (coverage.out + coverage.html)
	go test ./... -race -covermode=atomic -coverprofile=$(COVER_FILE)
	go tool cover -func=$(COVER_FILE) | tail -1
	go tool cover -html=$(COVER_FILE) -o coverage.html

lint: ## Roda o golangci-lint
	golangci-lint run

fmt: ## Formata o código (gofmt -s)
	gofmt -s -w .

vet: ## Roda o go vet
	go vet ./...

tidy: ## Ajusta go.mod/go.sum
	go mod tidy

docker-build: ## Constrói a imagem Docker da aplicação
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE) .

up: ## Sobe toda a stack (build + up -d)
	docker compose up -d --build

down: ## Derruba a stack
	docker compose down

clean: ## Remove artefatos de build e cobertura
	rm -rf bin $(COVER_FILE) coverage.html
