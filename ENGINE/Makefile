# ============================================================================
# VUKA Engine Makefile (Phase 1 Go engine + Phase 2 Python ISO gateway)
#
# Targets:
#   make build              compile the Go engine binary
#   make vet                run go vet
#   make test               run the full Go test suite against TEST_DATABASE_URL
#                           (defaults to the local dockerized PG on :5433,
#                            database vuka_test)
#   make test-integration   end-to-end Go: spin up dockerized PG, (re)provision
#                           a clean vuka_test schema, run the suite under
#                           `go test -race`
#   make gateway            build the Python ISO 20022 gateway venv
#   make gateway-test       run pytest against a LIVE Go engine (assumes the
#                           engine is running with gRPC on VUKA_ENGINE_GRPC_ADDR)
#   make up / make down     start / stop dockerized PostgreSQL + Adminer
#   make clean              drop the vuka_test database
# ============================================================================

SHELL := /bin/bash
COMPOSE_FILE := deploy/docker-compose.yml
PG_SERVICE := postgres
# host port mapped to container 5432
PG_HOST_PORT := 5433
PG_USER := vuka
# Local dev credential — overridable via env; never a committed secret.
# Set PG_PASSWORD in your shell (or .env) with your local dev password.
PG_PASSWORD ?= changeme
# DB created by the container on start
PG_SUPER_DB := vuka
PG_TEST_DB := vuka_test

# The DSN the Go tests run against. Point here to control the target database.
TEST_DATABASE_URL ?= postgres://$(PG_USER):$(PG_PASSWORD)@localhost:$(PG_HOST_PORT)/$(PG_TEST_DB)?sslmode=disable

MKFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

.PHONY: build vet test test-integration up down clean psql provision

build:
	cd $(MKFILE_DIR)go-bridge-core && go build ./...

vet:
	cd $(MKFILE_DIR)go-bridge-core && go vet ./...

test:
	cd $(MKFILE_DIR)go-bridge-core && TEST_DATABASE_URL='$(TEST_DATABASE_URL)' go test ./internal/... -count=1

# Deterministic container name for psql/pg_isready/copy targets.
PG_CONTAINER := $(shell docker compose -f $(MKFILE_DIR)$(COMPOSE_FILE) ps -q $(PG_SERVICE) 2>/dev/null | head -n1)

up:
	docker compose -f $(MKFILE_DIR)$(COMPOSE_FILE) up -d

down:
	docker compose -f $(MKFILE_DIR)$(COMPOSE_FILE) down

# (Re)provision a clean vuka_test database from deploy/init.sql inside the
# running postgres container. Idempotent: drops any previous test DB first.
provision:
	@test -n "$(PG_CONTAINER)" || { echo "ERROR: postgres container is not running; run 'make up' first."; exit 1; }
	@echo "Provisioning clean $(PG_TEST_DB) database..."
	@docker exec "$(PG_CONTAINER)" psql -U $(PG_USER) -d $(PG_SUPER_DB) -v ON_ERROR_STOP=1 \
		-c "DROP DATABASE IF EXISTS $(PG_TEST_DB);" \
		-c "CREATE DATABASE $(PG_TEST_DB);"
	docker exec -i "$(PG_CONTAINER)" psql -U $(PG_USER) -d $(PG_TEST_DB) -v ON_ERROR_STOP=1 \
		< $(MKFILE_DIR)deploy/init.sql
	@echo "Schema applied to $(PG_TEST_DB)."

# Integration harness: bring PG up, wait until healthy, provision, then test.
test-integration: up provision
	@echo "Waiting for PostgreSQL to become healthy..."
	@for i in $$(seq 1 30); do \
		if docker exec "$(PG_CONTAINER)" pg_isready -U $(PG_USER) -d $(PG_SUPER_DB) >/dev/null 2>&1; then \
			echo "PostgreSQL is up."; break; \
		fi; \
		sleep 1; \
	done
	$(MAKE) test

clean: down
	@test -n "$(PG_CONTAINER)" || exit 0
	docker exec "$(PG_CONTAINER)" psql -U $(PG_USER) -d $(PG_SUPER_DB) \
		-c "DROP DATABASE IF EXISTS $(PG_TEST_DB);"

psql:
	@test -n "$(PG_CONTAINER)" || { echo "ERROR: postgres container is not running; run 'make up' first."; exit 1; }
	docker exec -it "$(PG_CONTAINER)" psql -U $(PG_USER) -d $(PG_TEST_DB)

# ---------------------------------------------------------------------------
# Phase 2 — Python ISO 20022 gateway
# ---------------------------------------------------------------------------

GATEWAY_DIR := $(MKFILE_DIR)python-iso-gateway
GATEWAY_VENV := $(GATEWAY_DIR)/.venv/bin
GATEWAY_PROTO := $(MKFILE_DIR)proto/vuka.proto

# Rebuild the generated gRPC stubs from the single source of truth.
proto-python:
	cd $(GATEWAY_DIR) && .venv/bin/python3 -m grpc_tools.protoc \
		-I$(MKFILE_DIR)proto --python_out=. --grpc_python_out=. $(GATEWAY_PROTO)

# Create the venv and install gateway dependencies.
gateway:
	cd $(GATEWAY_DIR) && test -d .venv || python3 -m venv .venv
	cd $(GATEWAY_DIR) && .venv/bin/pip install --quiet -e '.[dev]'
	$(MAKE) proto-python

# Run the Python test suite. Requires a live Go engine on
# VUKA_ENGINE_GRPC_ADDR (default localhost:50051).
VUKA_ENGINE_GRPC_ADDR ?= localhost:50051
gateway-test: gateway
	cd $(GATEWAY_DIR) && VUKA_ENGINE_GRPC_ADDR='$(VUKA_ENGINE_GRPC_ADDR)' \
		.venv/bin/python3 -m pytest -q

# Start the gateway with uvicorn (dev server; engine must already run).
gateway-run: gateway
	cd $(GATEWAY_DIR) && .venv/bin/uvicorn app.main:app --host 0.0.0.0 --port 8082

.PHONY: proto-python gateway gateway-test gateway-run

# ---------------------------------------------------------------------------
# Phase 3 — React/TS trader dashboard (FRONTEND/web-dashboard)
# ---------------------------------------------------------------------------

DASHBOARD_DIR := $(MKFILE_DIR)../FRONTEND/web-dashboard

# Install dashboard dependencies (npm). Run once after cloning.
dashboard-install:
	cd $(DASHBOARD_DIR) && npm install

# Typecheck + build the dashboard for production (deps must be installed).
dashboard-build:
	cd $(DASHBOARD_DIR) && npm run build

# Run the dashboard test suite (deps must be installed).
dashboard-test:
	cd $(DASHBOARD_DIR) && NODE_ENV=test npm test -- --reporter dot

# Start the dashboard dev server (proxies /api to the Go engine on :8080).
# Deps must be installed first; requires the Go engine running.
dashboard-run:
	cd $(DASHBOARD_DIR) && npm run dev

# ---------------------------------------------------------------------------
# Live demo: seed demo traders, start engine + dashboard together
# ---------------------------------------------------------------------------

# Seed the demo traders into the running engine (idempotent via by-phone).
# Reads TEST_DATABASE_URL; falls back to the compose PG like `make test`.
DEMO_SEED_URL ?= $(TEST_DATABASE_URL)
demo-seed:
	@echo "Seeding demo traders into $(DEMO_SEED_URL)…"
	cd $(MKFILE_DIR)go-bridge-core && VUKA_DATABASE_URL='$(DEMO_SEED_URL)' \
		go run ./cmd/seed

# One-command live demo: engine (background) + dashboard (background).
# Prereqs: dockerized PG up (make up), deps installed (make dashboard-install).
demo: up
	@echo "=== VUKA live demo ==="
	@echo "1. Seeding demo traders…"
	$(MAKE) demo-seed
	@echo "2. Starting Go engine on :8080 (gRPC :50051)…"
	cd $(MKFILE_DIR)go-bridge-core && VUKA_DATABASE_URL='$(DEMO_SEED_URL)' \
		VUKA_HTTP_ADDR=':8080' VUKA_GRPC_ADDR=':50051' MTN_SIM_MODE=success \
		MTN_SIM_DELAY_MS=5 VUKA_FX_RWF_KES=9.5 \
		VUKA_CORS_ORIGINS='http://localhost:5173' \
		go run ./cmd/ > /tmp/vuka-engine.log 2>&1 &
	@echo "3. Starting dashboard on http://localhost:5173…"
	cd $(DASHBOARD_DIR) && npm run dev > /tmp/vuka-dashboard.log 2>&1 &
	@echo "Open http://localhost:5173 — pick a demo trader and transact."
	@echo "Engine log: /tmp/vuka-engine.log | Dashboard log: /tmp/vuka-dashboard.log"
	@sleep 2 && curl -s http://localhost:8080/healthz && echo " (engine healthy)"

.PHONY: dashboard-install dashboard-build dashboard-test dashboard-run demo demo-seed