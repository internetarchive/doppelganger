# Start from the official Go image
FROM golang:1.26.2-alpine AS builder

# Optional build args for Go module proxy, checksum database, and HTTP proxies
ARG GOPROXY
ARG GOSUMDB
ARG http_proxy
ARG https_proxy

# Set the working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN go build -o doppelganger .

# Start a new stage from scratch
FROM alpine:latest  

# Optional build args for HTTP proxies
ARG http_proxy
ARG https_proxy

RUN apk --no-cache --no-check-certificate add ca-certificates curl

WORKDIR /root/

# Copy the pre-built binary file from the previous stage
COPY --from=builder /app/doppelganger .

# Command to run the executable
CMD ["./doppelganger", "server"]
