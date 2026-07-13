DOMAIN ?= localhost

.PHONY: setup up down logs test build clean smoke-test test-backend-unit test-backend-api test-backend-docker test-frontend-static test-frontend-unit test-e2e test-e2e-prepare test-live-smoke frontend-dev frontend-build sdlc-prepare sdlc-smoke sdlc-teardown

-include .env
export

setup:
	@test -f .env || cp .env.example .env
	@echo "Edit .env and run: make up"

build:
	docker compose build

up:
	@test -f .env || cp .env.example .env
	docker compose build agent egress-proxy
	docker compose up -d
	@echo ""
	@echo "Frontend: https://$(DOMAIN)"; \
	echo "API:      https://$(DOMAIN)/api"

down:
	docker compose down

logs:
	docker compose logs -f backend

## Fast, Docker-free backend test lane. `make test` remains its compatibility alias.
test: test-backend-unit

test-backend-unit:
	go test ./backend/...

## Isolated API checks. Each script owns its temp database, port, and cleanup.
test-backend-api:
	@for command in go curl git sqlite3; do \
		command -v "$$command" >/dev/null 2>&1 || { echo "test-backend-api requires $$command on PATH" >&2; exit 2; }; \
	done
	@env -u PORT ./backend/scripts/test-auth.sh
	@env -u PORT ./backend/scripts/test-projects.sh

## Docker integration. Requires a fresh, job-owned daemon and explicit acknowledgement.
test-backend-docker:
	@./scripts/test-backend-docker.sh

## Fast, Docker-free frontend static lane.
test-frontend-static:
	@cd frontend && npm run lint && npm run build:offline

## Implemented by FEAT-052; kept as the stable frontend unit-test entry point.
test-frontend-unit:
	@cd frontend && npm run test:unit

## TEST-023 builder-only fixture preparation. It prints E2E_HANDOFF_FILE and
## leaves exact resources in E2E_MANIFEST for final builder cleanup.
test-e2e-prepare:
	@test -n "$(E2E_MANIFEST)" || (echo "test-e2e-prepare requires E2E_MANIFEST from the TEST-023 builder" >&2; exit 2)
	@E2E_MODE=prepare ./scripts/test-playwright-e2e.sh

## TEST-023 tester-only browser execution against the builder handoff. It
## never starts/selects a stack and must receive the exact handoff file.
test-e2e:
	@test -n "$(E2E_MANIFEST)" || (echo "test-e2e requires E2E_MANIFEST from the TEST-023 builder" >&2; exit 2)
	@test -n "$(E2E_HANDOFF_FILE)" || (echo "test-e2e requires E2E_HANDOFF_FILE from test-e2e-prepare" >&2; exit 2)
	@E2E_MODE=test ./scripts/test-playwright-e2e.sh

## Never starts, stops, or selects a stack. Run only against an operator-owned disposable stack.
test-live-smoke:
	@test -n "$(CADDY_HOST)" || (echo "test-live-smoke requires CADDY_HOST for the target stack" >&2; exit 2)
	@test -n "$(ADMIN_PASSWORD)" || (echo "test-live-smoke requires ADMIN_PASSWORD for the target stack" >&2; exit 2)
	@CADDY_HOST="$(CADDY_HOST)" ADMIN_PASSWORD="$(ADMIN_PASSWORD)" ./scripts/smoke-test.sh

## Deprecated compatibility name. It has the same explicit live-stack safeguards.
smoke-test: test-live-smoke

# Local, visual-only frontend preview. It does not start Docker or call Tamga APIs.
frontend-dev:
	@cd frontend && env -u PORT npm run dev:offline -- --port 3000

frontend-build:
	@cd frontend && env -u PORT npm run build:offline

sdlc-prepare:
	@test -n "$(MANIFEST)" || (echo "Usage: make sdlc-prepare MANIFEST=/tmp/tamga-sdlc-<task>.manifest" >&2; exit 2)
	@./scripts/sdlc-environment.sh prepare "$(MANIFEST)" "$(CURDIR)"

sdlc-smoke:
	@test -n "$(MANIFEST)" || (echo "Usage: make sdlc-smoke MANIFEST=/tmp/tamga-sdlc-<task>.manifest" >&2; exit 2)
	@./scripts/sdlc-environment.sh smoke "$(MANIFEST)" "$(CURDIR)"

sdlc-teardown:
	@test -n "$(MANIFEST)" || (echo "Usage: make sdlc-teardown MANIFEST=/tmp/tamga-sdlc-<task>.manifest" >&2; exit 2)
	@./scripts/sdlc-environment.sh cleanup "$(MANIFEST)"

clean: down
	docker compose down -v
