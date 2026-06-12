# ==========================================
# STAGE 1: Compilation Engine
# ==========================================
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git gcc musl-dev
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o aos cmd/aos/main.go

# ==========================================
# STAGE 2: Micro-Runtime Environment
# ==========================================
FROM alpine:3.20

# Runtime dependencies
RUN apk add --no-cache \
    openssh-server \
    bash \
    ca-certificates \
    tmux \
    util-linux

# Generate SSH host keys
RUN ssh-keygen -A

# Configure SSH — no hardcoded password here
RUN sed -i 's/#PermitRootLogin prohibit-password/PermitRootLogin yes/' /etc/ssh/sshd_config && \
    sed -i 's/#PasswordAuthentication yes/PasswordAuthentication yes/' /etc/ssh/sshd_config

# Copy compiled binary
COPY --from=builder /app/aos /usr/local/bin/aos
RUN chmod +x /usr/local/bin/aos

# Create workspace directory for agent file sandboxing
RUN mkdir -p /root/workspace

WORKDIR /root/

# Auto-launch AgentOS when user SSHs in
RUN echo 'if [ -x /usr/local/bin/aos ]; then exec /usr/local/bin/aos; fi' >> /root/.bashrc

# Entrypoint script — sets SSH password from env at runtime, then starts sshd
COPY entry.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

EXPOSE 22 8088

CMD ["/usr/local/bin/docker-entrypoint.sh"]