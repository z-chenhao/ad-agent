.PHONY: cli build test test-web
cli:
	npm run build --workspace=@ad-agent/pi-bridge
	go build -o bin/ad-agent ./cmd/ad-agent
build:
	npm run build
	go build -o bin/ad-agent ./cmd/ad-agent
test:
	go test -race ./...
	npm run build --workspace=@ad-agent/pi-bridge
	npm test --workspace=@ad-agent/pi-bridge
test-web: build
	npm run test:e2e --workspace=@ad-agent/web
