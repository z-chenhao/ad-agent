.PHONY: cli build test test-sandbox test-web test-web-portfolio
cli:
	npm run build --workspaces --if-present
	go build -o bin/ad-agent ./cmd/ad-agent
build:
	npm run build
	go build -o bin/ad-agent ./cmd/ad-agent
test:
	./scripts/check-english.sh
	go test -race ./...
	npm run build --workspaces --if-present
	npm test --workspaces --if-present
test-sandbox:
	go test -race ./internal/sandbox ./internal/app
test-web: build
	npm run test:e2e --workspace=@ad-agent/web
test-web-portfolio: build
	npm run test:e2e:portfolio --workspace=@ad-agent/web
