.PHONY: setup up down logs test build clean

DOMAIN ?= localhost

setup:
	@test -f .env || cp .env.example .env
	@echo "Edit .env and run: make up"

build:
	docker build -t tamga-backend -f deploy/Dockerfile.backend .
	docker build -t tamga-frontend -f deploy/Dockerfile.frontend .
	docker build -t tamga-agent -f deploy/Dockerfile.agent .

up: build
	docker network inspect tamga-net >/dev/null 2>&1 || docker network create tamga-net
	docker run -d --name tamga-caddy \
		--network tamga-net \
		-p 80:80 -p 443:443 -p 2019:2019 \
		-v /var/run/docker.sock:/var/run/docker.sock:ro \
		-v caddy_data:/data \
		-e DOMAIN=$(DOMAIN) \
		caddy:2-alpine \
		caddy run --config /etc/caddy/Caddyfile --adapter caddyfile
	docker run -d --name tamga-backend \
		--network tamga-net \
		-v /var/run/docker.sock:/var/run/docker.sock:ro \
		-v tamga_data:/data \
		--env-file .env \
		tamga-backend
	docker run -d --name tamga-frontend \
		--network tamga-net \
		--env-file .env \
		tamga-frontend

down:
	-docker rm -f tamga-caddy tamga-backend tamga-frontend 2>/dev/null

logs:
	docker logs -f tamga-backend

test:
	go test ./backend/...

build-backend:
	CGO_ENABLED=0 go build -o bin/api ./backend/cmd/api/

run-backend:
	go run ./backend/cmd/api/

clean: down
	docker network rm tamga-net 2>/dev/null; true
	docker volume rm caddy_data tamga_data 2>/dev/null; true
