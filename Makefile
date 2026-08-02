.PHONY: dev api web migrate tidy test build

dev:
	@echo "Start postgres (compose) then: make migrate && make api (and make web in another terminal)"

tidy:
	go mod tidy

migrate:
	go run ./cmd/dockfin migrate

api:
	go run ./cmd/dockfin serve

web:
	cd apps/web && npm run dev

build:
	go build -o bin/dockfin.exe ./cmd/dockfin
	go build -o bin/dfin.exe ./cmd/dfin
	go build -o bin/dockfin-sentinel.exe ./cmd/dockfin-sentinel

test:
	go test ./...

compose-up:
	docker compose -f deploy/compose/docker-compose.yml up -d --build
