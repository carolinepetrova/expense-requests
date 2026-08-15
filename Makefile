.PHONY: install-tools generate tidy build run run-multi-step test web web-build

# go-enum generates the enum constants, parsers and marshallers.
# The generated *_enum.go files are committed, so this is only needed after
# changing an ENUM comment.
install-tools:
	go install github.com/abice/go-enum@latest
	go install github.com/onsi/ginkgo/v2/ginkgo@latest

generate:
	go generate ./...

tidy:
	go mod tidy

build:
	go build -o bin/server ./cmd/server

run:
	go run ./cmd/server -data ./data -addr :8080

# The optional extension: expenses of $$1,000 or more go to the manager first
# and then to finance.
run-multi-step:
	go run ./cmd/server -data ./data -addr :8080 -multi-step

test:
	go test ./...

# Frontend
web:
	cd web && npm install && npm run dev

web-build:
	cd web && npm install && npm run build
