DOMAIN ?= localhost
CADDY_EMAIL ?= admin@example.com

.PHONY: setup up down logs test build clean

-include .env
export

setup:
	@test -f .env || cp .env.example .env
	@echo "Edit .env and run: make up"

build:
	docker build -t tamga-backend -f deploy/Dockerfile.backend .
	docker build -t tamga-frontend -f deploy/Dockerfile.frontend .
	docker build -t tamga-agent -f deploy/Dockerfile.agent .

network:
	docker network inspect tamga-net >/dev/null 2>&1 || docker network create tamga-net

up: network build
	@test -f .env || cp .env.example .env
	$(eval SYSTEM_CODE_DIR := $(shell pwd))
	docker run -d --name caddy \
		--network tamga-net \
		-p 80:80 -p 443:443 -p 2019:2019 \
		-v caddy_data:/data \
		-v ./deploy/Caddyfile:/etc/caddy/Caddyfile:ro \
		-e DOMAIN=$(DOMAIN) \
		-e CADDY_EMAIL=$(CADDY_EMAIL) \
		caddy:2-alpine
	docker run -d --name tamga-backend \
		--network tamga-net \
		-v /var/run/docker.sock:/var/run/docker.sock:ro \
		-v tamga_data:/data \
		-v $(SYSTEM_CODE_DIR):$(SYSTEM_CODE_DIR):ro \
		-e SYSTEM_CODE_DIR=$(SYSTEM_CODE_DIR) \
		--env-file .env \
		tamga-backend
	docker run -d --name tamga-frontend \
		--network tamga-net \
		--env-file .env \
		-e PORT=3000 \
		tamga-frontend
	@echo ""
	@echo "Frontend: https://$(DOMAIN)"; \
	echo "API:      https://$(DOMAIN)/api"

down:
	-docker rm -f caddy tamga-backend tamga-frontend 2>/dev/null

logs:
	@docker logs -f tamga-backend 2>&1 || true

test:
	go test ./backend/...

clean: down
	-docker network rm tamga-net 2>/dev/null
	-docker volume rm caddy_data tamga_data 2>/dev/null
