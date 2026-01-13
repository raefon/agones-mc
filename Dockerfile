# Use Go 1.24 on Alpine for a small, modern build environment
FROM golang:1.24-alpine AS build

# Install make and ca-certificates (required for building and HTTPS)
RUN apk add --no-cache make git ca-certificates

WORKDIR /agones-mc/

# Copy module files first to leverage Docker layer caching
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 is required for the binary to run in a 'scratch' image
# (otherwise it looks for libraries that don't exist in scratch)
RUN CGO_ENABLED=0 make build

# Final Stage
FROM scratch
WORKDIR /agones-mc/

# Copy certificates so HTTPS requests work
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Copy the binary from the build stage
COPY --from=build /agones-mc/build/agones-mc .

ENTRYPOINT [ "./agones-mc" ]