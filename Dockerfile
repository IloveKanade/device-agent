# Multi-stage Docker build for Wails application

# Stage 1: Frontend build
FROM node:18-alpine AS frontend-builder

WORKDIR /app/frontend

# Copy package files
COPY frontend/package*.json ./

# Install dependencies
RUN npm ci --only=production

# Copy frontend source
COPY frontend/ ./

# Build frontend
RUN npm run build

# Stage 2: Go build
FROM golang:1.23-alpine AS backend-builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev linux-headers

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Copy built frontend from previous stage
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o device-agent .

# Stage 3: Final runtime image
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=backend-builder /app/device-agent .

# Expose port (adjust if your app uses a different port)
EXPOSE 8080

# Command to run the executable
CMD ["./device-agent"]