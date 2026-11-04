.PHONY: dev check check-go check-frontend

dev:
	python3 scripts/dev_env.py
	docker compose --profile test up --build -d

check-go:
	docker run --rm -v "$(CURDIR)/backend:/src" -v campusclaw-team-gomod:/go/pkg/mod -v campusclaw-team-gocache:/root/.cache/go-build -w /src golang:1.23-alpine sh -c 'go vet ./... && go test ./... && go build -o /tmp/api ./cmd/server'
	docker run --rm -v "$(CURDIR)/mcp-server:/src" -w /src golang:1.23-alpine sh -c 'go vet ./... && go test ./...'

check-frontend:
	npm --prefix frontend run build
	node --experimental-strip-types --test frontend/tests/*.test.mjs

check: check-go check-frontend
	openspec validate 2026-11-04-async-grading --strict
	git diff --check
