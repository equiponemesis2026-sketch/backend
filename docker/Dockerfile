# Step 1: Build the binary in a secure builder image
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install system dependencies
RUN apk add --no-cache git ca-certificates

# Copy dependency files and pre-fetch
COPY go.mod go.sum ./
RUN go mod download

# Copy application source
COPY . .

# Build security-hardened statically linked binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -extldflags '-static'" \
    -o /app/nemesis-api cmd/nemesis-api/main.go

# Step 2: Run in a minimal scratch/distroless-like environment to minimize attack surface
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -g '' appuser

WORKDIR /app

COPY --from=builder /app/nemesis-api .

# Run as non-root user
USER appuser

EXPOSE 8080

ENTRYPOINT ["./nemesis-api"]
