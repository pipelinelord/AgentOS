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

RUN apk add --no-cache openssh-server bash ca-certificates tmux util-linux

# Configure internal OpenSSH isolation limits
RUN ssh-keygen -A
RUN echo 'root:AgentOS_Secure_Token_2026!' | chpasswd
RUN sed -i 's/#PermitRootLogin prohibit-password/PermitRootLogin yes/' /etc/ssh/sshd_config
RUN sed -i 's/#PasswordAuthentication yes/PasswordAuthentication yes/' /etc/ssh/sshd_config

WORKDIR /root/
COPY --from=builder /app/aos /usr/local/bin/aos

# Force active user session bindings to lock directly into the AgentOS console UI
RUN echo "if [ -x /usr/local/bin/aos ]; then exec /usr/local/bin/aos; fi" >> /root/.bashrc

EXPOSE 22
CMD ["/usr/sbin/sshd", "-D", "-e"]
