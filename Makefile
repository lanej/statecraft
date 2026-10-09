.PHONY: api web generate check-generated proto-lint proto-breaking format-go gofmt-check format check-format

PROTO_BASE ?= main

api:
	go run ./cmd/statecraft

web:
	cd web && npm run dev

generate:
	web/node_modules/.bin/buf generate

check-generated: generate
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.1 --config api/github/codegen.yaml api/github/read-only.json
	git diff --exit-code -- internal/adapters/githubread/generated
	git diff --exit-code -- gen web/src/gen
	test -z "$$(git ls-files --others --exclude-standard gen web/src/gen)"

proto-lint:
	web/node_modules/.bin/buf lint

proto-breaking:
	web/node_modules/.bin/buf breaking --against '.git#branch=$(PROTO_BASE)'

format-go:
	gofmt -w .

gofmt-check:
	sh scripts/check-gofmt.sh

format: format-go
	npm --prefix web run format

check-format: gofmt-check
	npm --prefix web run check

.PHONY: readonly build test check github-spec
readonly:
	go run ./cmd/statecraft-readonly --local

build:
	npm --prefix web ci
	npm --prefix web run build
	go build ./cmd/statecraft ./cmd/statecraft-readonly

test:
	go test ./...
	npm --prefix web test

check: check-format proto-lint test
	npm --prefix web run build

github-spec:
	node scripts/github-spec.mjs
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.1 --config api/github/codegen.yaml api/github/read-only.json
