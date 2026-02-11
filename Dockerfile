# Stage 1: Build Go tools. The module cache stays in this stage.
FROM golang:1.25.6-bookworm AS go-tools

ARG MAGE_VERSION=1.15.0
ARG BEADS_VERSION=0.49.3
ARG GOLANGCI_LINT_VERSION=2.1.6

RUN go install github.com/magefile/mage@v${MAGE_VERSION} && \
    go install github.com/steveyegge/beads/cmd/bd@v${BEADS_VERSION} && \
    go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v${GOLANGCI_LINT_VERSION}

# Stage 2: Final image with only the runtime pieces.
FROM node:20-slim

ARG CLAUDE_CODE_VERSION=latest
ARG GO_VERSION=1.25.6

RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    ca-certificates \
    wget \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# Go runtime (needed to build the project).
RUN ARCH=$(dpkg --print-architecture) && \
    wget -q "https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz" && \
    tar -C /usr/local -xzf "go${GO_VERSION}.linux-${ARCH}.tar.gz" && \
    rm "go${GO_VERSION}.linux-${ARCH}.tar.gz"
ENV PATH="/usr/local/go/bin:/root/go/bin:${PATH}"
ENV GOPATH="/root/go"

# Copy only the compiled binaries from the builder stage.
COPY --from=go-tools /go/bin/mage /root/go/bin/
COPY --from=go-tools /go/bin/bd /root/go/bin/
COPY --from=go-tools /go/bin/golangci-lint /root/go/bin/

# Claude Code.
RUN npm install -g @anthropic-ai/claude-code@${CLAUDE_CODE_VERSION} && \
    npm cache clean --force

RUN mkdir -p /root/.claude

WORKDIR /workspace

COPY docker-entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["bash"]
