FROM node:20-alpine AS web

WORKDIR /web
COPY web/package.json ./
RUN npm install

COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS server

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server

FROM alpine:3.20

WORKDIR /app
COPY --from=server /server ./server
COPY --from=web /web/dist ./web
COPY data/ ./data/

EXPOSE 8080

ENTRYPOINT ["/app/server", "-addr", ":8080", "-data", "/app/data", "-web-dir", "/app/web"]
