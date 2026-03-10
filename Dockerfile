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
RUN CGO_ENABLED=0 go mod download

# Copy the source code
COPY . .

# Build the main application binary
# -ldflags to embed version info and strip debug symbols (-s -w) for smaller binary
# Output binary to /geoip-server for easy copying in the next stage
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X 'main.BuildVersion=${BUILD_VERSION}' -X 'main.GitCommit=${GIT_COMMIT}'" -o /geoip-server ./

# Create db directories with correct ownership for the non-root user.
# Chainguard static uses UID 65532 (nonroot).
RUN mkdir -p /db/v1 /db/v2 && chown -R 65532:65532 /db

# Stage 2: Hardened runtime image
# Chainguard static: zero known CVEs, rebuilt nightly, non-root by default,
# includes CA certificates for HTTPS. No shell, no package manager.
FROM cgr.dev/chainguard/static:latest

# Set working directory for the application
WORKDIR /app

# Copy the built executable from the builder stage
COPY --from=builder /geoip-server /app/geoip-server

# Copy the pre-created /db directory with correct ownership
COPY --from=builder /db /db

# Expose the web port
EXPOSE 7502

# Define the HEALTHCHECK using the binary's subcommand.
HEALTHCHECK --start-period=60s --interval=60s --timeout=5s --retries=3 \
  CMD ["/app/geoip-server", "healthcheck"]

# Set the entry point to run our application
ENTRYPOINT ["/app/geoip-server"]