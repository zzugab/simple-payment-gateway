# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.mod ./
COPY . .
# Generate and verify go.sum from the module requirements and source imports.
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# Runtime stage
FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
    && addgroup -S app \
    && adduser -S -G app app

WORKDIR /app
COPY --from=builder /out/api ./api

USER app:app
EXPOSE 8080

ENTRYPOINT ["./api"]
