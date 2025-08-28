# syntax=docker/dockerfile:1

ARG GO_VERSION=1.24

FROM golang:${GO_VERSION} AS builder
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    LANG=C.UTF-8 \
    LC_ALL=C.UTF-8 \
    TZ=Europe/Berlin

WORKDIR /src

# Build args for versioning
ARG VERSION=dev
ARG COMMIT=dev
ARG BUILD_TIME=dev

# Cache go mod
COPY go.mod ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download || true

# Copy source
COPY . .

# Stelle sicher, dass go.sum generiert wird und Abhängigkeiten konsistent sind
RUN --mount=type=cache,target=/go/pkg/mod go mod tidy

# Build
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w -X 'nalaerp3/internal/version.Version=${VERSION}' -X 'nalaerp3/internal/version.Commit=${COMMIT}' -X 'nalaerp3/internal/version.BuildTime=${BUILD_TIME}'" -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12
ENV LANG=C.UTF-8 \
    LC_ALL=C.UTF-8 \
    TZ=Europe/Berlin
USER nonroot:nonroot
COPY --from=builder /out/api /api
EXPOSE 8080
ENTRYPOINT ["/api"]
