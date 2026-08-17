.PHONY: install-tools generate tidy lint lint-fix build run run-multi-step test \
	web web-build docker docker-multi-step

# Everything in one container, API and UI on :8080. Needs neither Go nor Node
# installed — this is the way to run it if you have neither.
docker:
	docker build -t expense-requests .
	docker run --rm -p 8080:8080 expense-requests

docker-multi-step:
	docker build -t expense-requests .
	docker run --rm -p 8080:8080 expense-requests -multi-step


# go-enum generates the enum constants, parsers and marshallers.
# The generated *_enum.go files are committed, so this is only needed after
# changing an ENUM comment.
install-tools:
	go install github.com/abice/go-enum@latest
	go install github.com/onsi/ginkgo/v2/ginkgo@latest
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.4.0

generate:
	go generate ./...

tidy:
	go mod tidy

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix

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
