.PHONY: build run dev test lint docker-build web-dist

web-dist:
	@test -f web/dist/index.html || (cd web && npm ci && npm run build)

build: web-dist
	go build -o bin/wizard ./cmd/wizard

run: build
	./bin/wizard

dev: web-dist
	WIZARD_ADDR=:8080 go run ./cmd/wizard

test: web-dist
	go test ./... -v

lint:
	golangci-lint run ./...

docker-build:
	docker build -t agentgateway-demo-wizard:local .

# Frontend dev server proxies /api to :8080 (see web/vite.config.ts). Run the
# Go backend in one terminal (`make dev`) and the Vite dev server in another:
#   cd web && npm run dev
