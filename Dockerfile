# Stage 1: Build the Go application
FROM golang:1.25-alpine3.22 AS builder

# Set build arguments for version info
ARG BUILD_VERSION=dev
ARG GIT_COMMIT=unknown

# Set working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies. This step is cached if go.mod/go.sum don't change.
# CGO_ENABLED=0 is important for static binaries compatible with distroless/static
RUN CGO_ENABLED=0 go mod download

# Copy the source code
COPY . .

# Build the main application binary
# -ldflags to embed version info into the binary
# Output binary to /geoip-server for easy copying in the next stage
RUN CGO_ENABLED=0 go build -ldflags "-X 'main.BuildVersion=${BUILD_VERSION}' -X 'main.GitCommit=${GIT_COMMIT}'" -o /geoip-server ./

# Stage 2: Create the final Distroless image
FROM gcr.io/distroless/static:latest

# Set working directory for the application
WORKDIR /app

# Copy the built executable from the builder stage
COPY --from=builder /geoip-server /app/geoip-server

# Expose the web port
EXPOSE 7502

# Define the HEALTHCHECK using the binary's subcommand.
# --start-period: gives the container time to initialize without failing the health check.
# --interval: how often to run the check.
# --timeout: how long to wait for the command to return.
# --retries: how many consecutive failures before marking as unhealthy.
HEALTHCHECK --start-period=60s --interval=60s --timeout=5s --retries=3 \
  CMD ["/app/geoip-server", "healthcheck"]

# Set the entry point to run our application
# The entrypoint will always be our main binary.
ENTRYPOINT ["/app/geoip-server"]

# No default CMD; geoip-server runs as server by default