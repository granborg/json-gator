FROM golang:1.21-alpine AS builder

# Set the working directory
WORKDIR /app

# Copy go.mod
COPY go.mod ./

# Copy the source code
COPY *.go ./

# Build the application
RUN go build -o server .

# Create a lightweight production image
FROM alpine:latest

# Install CA certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /app/server .

# Expose port 8080
EXPOSE 8080

# Copy the built-in configurations
COPY *.json ./

# Set environment variables (can be overridden at runtime)
ENV CONFIG_FILE_PATH=""
ENV CONFIG_FILE_URL=""

# Run the application
CMD ["./server"]