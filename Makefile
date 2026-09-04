.PHONY: cli build test test-web
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
test-web: build
	npm run test:e2e --workspace=@ad-agent/web
