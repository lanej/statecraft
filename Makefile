.PHONY: api web generate check-generated proto-lint proto-breaking format-go gofmt-check format check-format

PROTO_BASE ?= main

api:
	go run ./cmd/statecraft

web:
	cd web && npm run dev

generate:
	web/node_modules/.bin/buf generate

check-generated: generate
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
