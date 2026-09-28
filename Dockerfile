# Node.js build stage for webcomponent
FROM node:22-alpine AS node-builder

WORKDIR /build

# Copy package files
COPY package*.json vite.config.ts tsconfig.json ./

# Install dependencies
RUN npm ci

COPY webcomponent/ webcomponent/
COPY app/ app/
RUN npm run build

# Go build stage
FROM golang:1.27.0-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Copy built webcomponent from node-builder
COPY --from=node-builder /build/static ./static/
COPY ./static/* ./static/

# Build the application
# CGO_ENABLED=0 for static binary
# -ldflags="-w -s" to strip debug info and reduce binary size
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-w -s" \
  -o /build/profile-http \
  ./cmd/profile-http

# Final stage
FROM alpine:latest

ENV APP_ENV=production \
  GIN_MODE=release

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 appuser && \
  adduser -D -u 1000 -G appuser appuser

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/profile-http /app/profile-http

# Copy static files if needed
COPY --from=builder /build/static /app/static

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port (adjust as needed)
EXPOSE 8080

# Run the application
CMD ["/app/profile-http"]
