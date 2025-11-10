# Multi-stage build for Piano PIR Database Generator
# Stage 1: Build Go binary
FROM golang:1.21-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Copy Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY main.go ./

# Build binary with optimizations
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-w -s" -o db-generator .

# Stage 2: Minimal runtime image
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates

# Copy binary from builder
COPY --from=builder /build/db-generator /app/db-generator

# Output directory
VOLUME /data

# Run the generator
ENTRYPOINT ["/app/db-generator"]
