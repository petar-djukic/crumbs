FROM node:20-bookworm

ARG CLAUDE_CODE_VERSION=latest
ARG GO_VERSION=1.25.7
ARG MAGE_VERSION=1.15.0
ARG BEADS_VERSION=0.49.3
ARG GOLANGCI_LINT_VERSION=2.1.6

# System dependencies.
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    jq \
    sudo \
    less \
    procps \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# Go.
RUN ARCH=$(dpkg --print-architecture) && \
    wget -q "https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz" && \
    tar -C /usr/local -xzf "go${GO_VERSION}.linux-${ARCH}.tar.gz" && \
    rm "go${GO_VERSION}.linux-${ARCH}.tar.gz"
ENV PATH="/usr/local/go/bin:/root/go/bin:${PATH}"
ENV GOPATH="/root/go"

# Mage (build automation).
RUN go install github.com/magefile/mage@v${MAGE_VERSION}

# Beads (issue tracker).
RUN go install github.com/steveyegge/beads/cmd/bd@v${BEADS_VERSION}

# golangci-lint.
RUN go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v${GOLANGCI_LINT_VERSION}

# Claude Code.
RUN npm install -g @anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}

WORKDIR /workspace

# Claude Code reads ANTHROPIC_API_KEY at runtime.
# Pass it via: docker run -e ANTHROPIC_API_KEY=sk-...
# Or mount your config: docker run -v ~/.claude:/root/.claude

CMD ["bash"]
