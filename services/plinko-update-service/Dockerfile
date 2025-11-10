# Multi-stage build for Plinko Update Service
# Stage 1: Build Go binary
FROM golang:1.21-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Copy Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY *.go ./

# Build binary with optimizations
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-w -s" -o plinko-update .

# Stage 2: Minimal runtime image
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates curl

# Copy binary from builder
COPY --from=builder /build/plinko-update /app/plinko-update

# Output directory for deltas
VOLUME /data

# Expose health check port
EXPOSE 3001

# Run the update service
ENTRYPOINT ["/app/plinko-update"]
