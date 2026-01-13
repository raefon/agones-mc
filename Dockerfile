# Use Go 1.24 on Alpine
FROM golang:1.24-alpine AS build

# Install build tools
RUN apk add --no-cache make git ca-certificates

WORKDIR /agones-mc/

# Pre-copy modules to improve caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Fix: Ensure go.mod is tidy before building
RUN go mod tidy

# Accept VERSION from Makefile/GitHub Actions
ARG VERSION=dev
ARG ARCH=amd64

# Use the Makefile to build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} make build VERSION=${VERSION}

# Final Stage
FROM scratch
WORKDIR /agones-mc/
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /agones-mc/build/agones-mc .

ENTRYPOINT [ "./agones-mc" ]