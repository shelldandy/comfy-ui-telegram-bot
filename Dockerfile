# Build stage
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /build

# Copy dependency files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o comfy-tg-bot \
    ./cmd/bot

# Runtime stage
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/comfy-tg-bot /app/comfy-tg-bot

WORKDIR /app

USER nonroot:nonroot

ENTRYPOINT ["/app/comfy-tg-bot"]
