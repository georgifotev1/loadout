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

## up: start local infrastructure (Postgres, Mailpit, River UI on :8082) and wait until it is healthy
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

## mock-vali: run the fake Vali API on :8081 (make mock-vali args="-fail-rate 0.2 -latency 300ms")
.PHONY: mock-vali
mock-vali:
	go run ./cmd/mock-vali $(args)

## migrate: apply pending migrations, ours and River's (make migrate cmd=down|status for the others)
.PHONY: migrate
migrate:
	go run ./cmd/loadout migrate $(or $(cmd),up)

## sync: pull products from the supplier (mock-vali locally: run `make mock-vali` first) into the catalog
.PHONY: sync
sync:
	go run -ldflags "$(LDFLAGS)" ./cmd/loadout sync

## seed-specs: store the development spec values for builder parts (fixtures/specs.json); run after sync
.PHONY: seed-specs
seed-specs:
	go run ./cmd/loadout specs seed

## migration: create a new SQL migration (make migration name=create_catalog_products)
.PHONY: migration
migration:
	@test -n "$(name)" || (echo "usage: make migration name=<snake_case_name>" && exit 1)
	go run github.com/pressly/goose/v3/cmd/goose@v3.28.0 -dir migrations -s create $(name) sql

## gen: generate the Go server and the web app's API types from api/openapi.yaml
.PHONY: gen
gen:
	go tool oapi-codegen -config api/oapi-codegen.yaml -o cmd/loadout/api.gen.go api/openapi.yaml
	pnpm -C web gen:api

## gen-check: fail if the generated code is out of date with api/openapi.yaml
.PHONY: gen-check
gen-check:
	@tmp=$$(mktemp) && trap 'rm -f $$tmp' EXIT \
		&& go tool oapi-codegen -config api/oapi-codegen.yaml -o $$tmp api/openapi.yaml \
		&& cmp -s $$tmp cmd/loadout/api.gen.go \
		|| (echo "cmd/loadout/api.gen.go is stale: run make gen" && exit 1)
	@pnpm -C web --silent gen:api --check

## test: run all Go tests (integration tests need Docker)
.PHONY: test
test:
	go test -race ./...

## test-short: run Go tests that don't need Docker
.PHONY: test-short
test-short:
	go test -short ./...

## audit: format check, vet, staticcheck, tests, and web check (lint + format)/typecheck/test
.PHONY: audit
audit:
	@echo "==> gen-check"
	@$(MAKE) --no-print-directory gen-check
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
	pnpm -C web check
	pnpm -C web typecheck
	pnpm -C web test
