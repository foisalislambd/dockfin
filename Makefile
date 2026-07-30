.PHONY: dev api web migrate tidy test build

dev:
	@echo "Start postgres (compose) then: make migrate && make api (and make web in another terminal)"

tidy:
	go mod tidy

migrate:
	go run ./cmd/goolify migrate

api:
	go run ./cmd/goolify serve

web:
	cd apps/web && npm run dev

build:
	go build -o bin/goolify.exe ./cmd/goolify
	go build -o bin/glfy.exe ./cmd/glfy
	go build -o bin/goolify-sentinel.exe ./cmd/goolify-sentinel

test:
	go test ./...

compose-up:
	docker compose -f deploy/compose/docker-compose.yml up -d --build
