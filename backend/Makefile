.PHONY: run build tidy test fmt db-up db-down db-logs db-psql

run:
	go run ./cmd/server

db-up:
	docker compose up -d postgres

db-down:
	docker compose down

db-logs:
	docker compose logs -f postgres

db-psql:
	docker compose exec postgres psql -U interviewprep -d interviewprep

build:
	go build -o bin/server ./cmd/server

tidy:
	go mod tidy

test:
	go test ./...

fmt:
	go fmt ./...
