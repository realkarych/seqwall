FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY main.go ./
COPY pkg/ ./pkg/
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /usr/local/bin/seqwall .

FROM debian:bookworm-slim
ARG TARGETARCH
ARG DBMATE_VERSION=2.27.0
ENV DEBIAN_FRONTEND=noninteractive
RUN case "$TARGETARCH" in amd64|arm64) ;; *) echo "Unsupported architecture: $TARGETARCH" >&2; exit 1 ;; esac && \
    apt-get update && \
    apt-get install -y --no-install-recommends curl ca-certificates tzdata postgresql-common && \
    curl -fsSL https://github.com/amacneil/dbmate/releases/download/v${DBMATE_VERSION}/dbmate-linux-${TARGETARCH} \
      -o /usr/local/bin/dbmate && chmod +x /usr/local/bin/dbmate && \
    apt-get clean && rm -rf /var/lib/apt/lists/*
COPY --from=builder /usr/local/bin/seqwall /usr/local/bin/seqwall
COPY .ci/run_tests.sh /usr/local/bin/run_tests.sh
RUN chmod +x /usr/local/bin/run_tests.sh
WORKDIR /work
COPY test_data/ ./test_data/
ENTRYPOINT ["/usr/local/bin/run_tests.sh"]
