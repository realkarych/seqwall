########## 1. build stage ######################################################
FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS builder

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
WORKDIR /src

RUN --mount=type=cache,target=/go/pkg \
    --mount=type=cache,target=/root/.cache/go-build \
    true

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 \
    GOOS=$TARGETOS \
    GOARCH=$TARGETARCH \
    go build -trimpath \
      -ldflags="-s -w -X 'main.Version=${VERSION}'" \
      -o /out/seqwall .

FROM scratch AS tiny
LABEL org.opencontainers.image.source="https://github.com/realkarych/seqwall" \
      org.opencontainers.image.title="Seqwall" \
      org.opencontainers.image.description="Testing tool for PostgreSQL migrations" \
      org.opencontainers.image.licenses="MIT"

COPY --from=builder /out/seqwall /usr/bin/seqwall
ENTRYPOINT ["/usr/bin/seqwall"]
CMD ["--help"]

FROM alpine:3.20 AS debug
RUN apk add --no-cache bash postgresql-client ca-certificates tzdata
COPY --from=builder /out/seqwall /usr/bin/seqwall
ENTRYPOINT ["/usr/bin/seqwall"]
CMD ["--help"]
