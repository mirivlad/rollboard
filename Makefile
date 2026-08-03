.PHONY: dev backend frontend test test-unit fmt check smoke clean stop-dev build

dev:
	./scripts/dev.sh

backend:
	./scripts/backend.sh

frontend:
	./scripts/frontend.sh

# Runs the integration tests for real. Needs Docker for PostgreSQL and Redis.
test:
	./scripts/test.sh

# Pure unit tests only, for when Docker is unavailable. Integration tests skip
# themselves here, so this is not a substitute for `make test`.
test-unit:
	cd backend && go test ./... -count=1 -timeout 120s

fmt:
	cd backend && gofmt -w ./cmd ./internal

check:
	./scripts/check.sh

smoke:
	./scripts/smoke.sh

stop-dev:
	./scripts/stop-dev.sh

build:
	cd backend && go build -o ../build/rollboard-server ./cmd/server/
	cd frontend && npm run build

clean:
	rm -rf build/
	rm -rf frontend/dist/
	rm -rf data/*.db data/*.db-shm data/*.db-wal
