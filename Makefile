.PHONY: dev backend frontend test check smoke clean stop-dev

dev:
	./scripts/dev.sh

backend:
	./scripts/backend.sh

frontend:
	./scripts/frontend.sh

test:
	cd backend && go test ./... -count=1

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
