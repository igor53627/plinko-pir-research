# Multi-stage build for Piano PIR Hint Generator
# Stage 1: Build Go binary
FROM golang:1.21-alpine AS builder

WORKDIR /build

# Copy Go module files
COPY go.mod ./
RUN go mod download

# Copy source code
COPY main.go ./

# Build binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-w -s" -o hint-generator .

# Stage 2: Minimal runtime image
FROM alpine:latest

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/hint-generator /app/hint-generator

# Copy wrapper script (already executable)
COPY generate-hint.sh /app/generate-hint.sh

# Output directory
VOLUME /data

# Run the generator via wrapper script
ENTRYPOINT ["/app/generate-hint.sh"]
