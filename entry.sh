#!/bin/bash
set -e

# ==========================================
# AgentOS Docker Entrypoint
# Sets runtime config from environment
# before starting services
# ==========================================

# Set SSH root password from environment variable
# Falls back to a random password if not provided (printed to logs once)
if [ -z "$SSH_PASSWORD" ]; then
    GENERATED=$(cat /dev/urandom | tr -dc 'A-Za-z0-9' | head -c 24)
    echo "root:${GENERATED}" | chpasswd
    echo "=========================================="
    echo "  WARNING: SSH_PASSWORD not set."
    echo "  Generated one-time password: ${GENERATED}"
    echo "  Set SSH_PASSWORD in your .env to use your own."
    echo "=========================================="
else
    echo "root:${SSH_PASSWORD}" | chpasswd
    echo "[AgentOS] SSH password set from environment."
fi

echo "[AgentOS] Starting sshd..."
exec /usr/sbin/sshd -D -e