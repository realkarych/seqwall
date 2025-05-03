FROM golang:1.24-bookworm AS builder
WORKDIR /src
COPY go.* ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /usr/local/bin/seqwall .

FROM debian:bookworm-slim
ARG DBMATE_VERSION=2.27.0
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && \
    apt-get install -y --no-install-recommends curl ca-certificates tzdata postgresql-common && \
    curl -fsSL https://github.com/amacneil/dbmate/releases/download/v${DBMATE_VERSION}/dbmate-linux-amd64 \
      -o /usr/local/bin/dbmate && chmod +x /usr/local/bin/dbmate && \
    apt-get clean && rm -rf /var/lib/apt/lists/*
COPY --from=builder /usr/local/bin/seqwall /usr/local/bin/seqwall
COPY .ci/run_tests.sh /usr/local/bin/run_tests.sh
RUN chmod +x /usr/local/bin/run_tests.sh
WORKDIR /work
ENTRYPOINT ["/usr/local/bin/run_tests.sh"]
