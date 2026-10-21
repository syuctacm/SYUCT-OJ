# Build context: repository root (see deploy/compose.prod.yaml).
FROM golang:1.26.1-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN go build -trimpath -ldflags="-s -w" -o /out/syuct-oj-api ./cmd/api

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/syuct-oj-api /app/syuct-oj-api
RUN mkdir -p /app/data/judge-files
EXPOSE 8080
ENTRYPOINT ["/app/syuct-oj-api"]
