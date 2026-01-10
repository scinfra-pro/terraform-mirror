# Terraform Mirror - Dockerfile
# Multi-stage build for minimal image

# === Build stage ===
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum for dependency caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o tf-mirror .

# === Runtime stage ===
FROM alpine:3.19

# CA certificates for HTTPS
RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copy binary
COPY --from=builder /app/tf-mirror .

# Port
EXPOSE 8080

# Run
CMD ["./tf-mirror"]

