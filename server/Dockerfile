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

FROM debian:bookworm-slim
ENV LANG=C.UTF-8 \
    LC_ALL=C.UTF-8 \
    TZ=Europe/Berlin
RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      ca-certificates \
      imagemagick \
      inkscape \
      libreoffice \
      fonts-dejavu \
      ghostscript \
      librsvg2-bin \
      libwmf0.2-7 \
      libwmf-bin \
 && rm -rf /var/lib/apt/lists/*
WORKDIR /
COPY --from=builder /out/api /api
EXPOSE 8080
USER nobody
ENTRYPOINT ["/api"]
