# Build Stage
FROM golang:1.24-alpine AS build

ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETARCH=amd64

# Install git for go mod download
RUN apk add --no-cache git

WORKDIR /shove

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -o shove ./cmd/shove

# Final Stage
FROM alpine:3.21

# Add necessary packages
RUN apk --no-cache add \
    ca-certificates \
    tzdata

# Create non-root user and necessary directories
RUN adduser -D -H -h /server shove && \
    mkdir -p /server /etc/shove/apns/production /etc/shove/apns/sandbox && \
    chown -R shove:shove /server /etc/shove

WORKDIR /server

# Copy binary from build stage
COPY --from=build /shove/shove .

# Set ownership of the binary
RUN chown shove:shove /server/shove

# Switch to non-root user
USER shove

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8322/health || exit 1

ENTRYPOINT ["/server/shove"]
