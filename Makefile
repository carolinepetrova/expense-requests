.PHONY: install-tools generate run test test-web web build tidy

install-tools:
	go install github.com/abice/go-enum@latest
	go install github.com/onsi/ginkgo/v2/ginkgo@latest

generate:
	go generate ./...

tidy:
	go mod tidy

test:
	go test ./...
# Frontend
web:
	cd web && npm install && npm run dev

test-web:
	cd web && npm run test
