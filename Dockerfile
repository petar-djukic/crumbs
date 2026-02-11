# Stage 1: Build Go tools with CGO disabled for static binaries.
FROM golang:1.25.6-alpine AS go-tools

ARG MAGE_VERSION=1.15.0
ARG BEADS_VERSION=0.49.3
ARG GOLANGCI_LINT_VERSION=2.1.6

ENV CGO_ENABLED=0
RUN go install github.com/magefile/mage@v${MAGE_VERSION} && \
    go install github.com/steveyegge/beads/cmd/bd@v${BEADS_VERSION} && \
    go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v${GOLANGCI_LINT_VERSION}

# Stage 2: Final image.
FROM node:20-alpine

ARG CLAUDE_CODE_VERSION=latest

RUN apk add --no-cache git bash

# Go runtime copied from the official image (no download step).
COPY --from=golang:1.25.6-alpine /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:/root/go/bin:${PATH}"
ENV GOPATH="/root/go"

# Compiled tool binaries from the builder stage.
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
