# Loadout developer commands. `make help` lists them.

# Load .env (if present) and export it to every recipe, so `make run` sees DATABASE_URL etc.
-include .env
export

COMPOSE := docker compose --env-file $(or $(wildcard .env),/dev/null) -f deploy/compose/docker-compose.yml
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

.DEFAULT_GOAL := help

## help: list the targets
.PHONY: help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed -e 's/^## //' | column -t -s ':'

## up: start local infrastructure (Postgres, Mailpit) and wait until it is healthy
.PHONY: up
up:
	$(COMPOSE) up -d --wait

## down: stop local infrastructure (keeps data; `make nuke` also deletes volumes)
.PHONY: down
down:
	$(COMPOSE) down

## nuke: stop local infrastructure and delete its volumes
.PHONY: nuke
nuke:
	$(COMPOSE) down -v

## logs: follow infrastructure logs
.PHONY: logs
logs:
	$(COMPOSE) logs -f

## run: run the API on the host (loadout serve)
.PHONY: run
run:
	go run -ldflags "$(LDFLAGS)" ./cmd/loadout serve

## build: build the loadout binary into ./bin
.PHONY: build
build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/loadout ./cmd/loadout

## web: run the TanStack Start dev server on :3000 (proxies /api to :8080)
.PHONY: web
web:
	pnpm -C web dev

## migrate: apply pending migrations (make migrate cmd=down|status for the others)
.PHONY: migrate
migrate:
	go run ./cmd/loadout migrate $(or $(cmd),up)

## migration: create a new SQL migration (make migration name=create_catalog_products)
.PHONY: migration
migration:
	@test -n "$(name)" || (echo "usage: make migration name=<snake_case_name>" && exit 1)
	go run github.com/pressly/goose/v3/cmd/goose@v3.28.0 -dir migrations -s create $(name) sql

## gen: run code generators (nothing yet)
.PHONY: gen
gen:
	@# TODO(phase-4): oapi-codegen for Go and openapi-typescript for web/src/lib/api.
	@echo "gen: nothing to generate yet"

## test: run all Go tests (integration tests need Docker)
.PHONY: test
test:
	go test -race ./...

## test-short: run Go tests that don't need Docker
.PHONY: test-short
test-short:
	go test -short ./...

## audit: format check, vet, staticcheck, tests, and web lint/typecheck/test
.PHONY: audit
audit:
	@echo "==> gofmt"
	@test -z "$$(gofmt -l .)" || (gofmt -l . && echo "run: gofmt -w ." && exit 1)
	@echo "==> go mod tidy"
	go mod tidy -diff
	@echo "==> go vet"
	go vet ./...
	@echo "==> staticcheck"
	go tool staticcheck ./...
	@echo "==> go test"
	go test -race ./...
	@echo "==> web"
	pnpm -C web lint
	pnpm -C web typecheck
	pnpm -C web test
