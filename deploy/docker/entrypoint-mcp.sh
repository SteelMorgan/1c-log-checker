#!/bin/sh
set -e

SSH_KEY_SRC="/mnt/work_data/${ONEC_SSH_KEY_PATH}"
SSH_KEY="/tmp/id_ed25519"

run_mcp() {
    exec su-exec appuser:appgroup /app/mcp "$@"
}

if [ -z "$ONEC_VM_IP" ] || [ -z "$ONEC_VM_USER" ] || [ -z "$ONEC_TECHLOG_CONFIG_REMOTE" ]; then
    echo "[entrypoint] ONEC_VM_IP / ONEC_VM_USER / ONEC_TECHLOG_CONFIG_REMOTE not set - running MCP without sshfs"
    run_mcp "$@"
fi

cp "$SSH_KEY_SRC" "$SSH_KEY"
chmod 600 "$SSH_KEY"
trap "rm -f $SSH_KEY" EXIT

echo "[entrypoint] waiting for SSH on ${ONEC_VM_IP}..."
for i in $(seq 1 30); do
    if ssh -i "$SSH_KEY" -o StrictHostKeyChecking=no -o ConnectTimeout=2 \
           "${ONEC_VM_USER}@${ONEC_VM_IP}" "echo ok" >/dev/null 2>&1; then
        echo "[entrypoint] SSH is reachable"
        break
    fi
    if [ "$i" = "30" ]; then
        echo "[entrypoint] ERROR: cannot reach SSH on ${ONEC_VM_IP} after 60s"
        exit 1
    fi
    sleep 2
done

echo "[entrypoint] mounting techlog config: ${ONEC_VM_USER}@${ONEC_VM_IP}:${ONEC_TECHLOG_CONFIG_REMOTE} -> /app/techlog-config"
sshfs "${ONEC_VM_USER}@${ONEC_VM_IP}:${ONEC_TECHLOG_CONFIG_REMOTE}" "/app/techlog-config" \
    -o IdentityFile="$SSH_KEY" \
    -o StrictHostKeyChecking=no \
    -o UserKnownHostsFile=/dev/null \
    -o allow_other \
    -o uid=100 \
    -o gid=101 \
    -o umask=0022 \
    -o reconnect \
    -o ServerAliveInterval=15 \
    -o ServerAliveCountMax=3

if ! mountpoint -q "/app/techlog-config"; then
    echo "[entrypoint] ERROR: techlog config mount failed"
    exit 1
fi

echo "[entrypoint] techlog config mounted OK"
run_mcp "$@"
