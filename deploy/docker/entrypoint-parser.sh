#!/bin/sh
set -e

# ---------------------------------------------------------------------------
# sshfs mount helper for log-parser container
# Mounts remote directories from onec-infra VM via SSH/FUSE
# SSH key is provided via Docker secrets (/run/secrets/onec_ssh_key)
# ---------------------------------------------------------------------------

SSH_KEY_SRC="/mnt/work_data/${ONEC_SSH_KEY_PATH}"
SSH_KEY="/tmp/id_ed25519"

# --- main ---

if [ -z "$ONEC_VM_IP" ] || [ -z "$ONEC_VM_USER" ]; then
    echo "[entrypoint] ONEC_VM_IP / ONEC_VM_USER not set — running parser without sshfs"
    exec /app/parser "$@"
fi

# SSH requires private key not accessible by others (max 0600)
cp "$SSH_KEY_SRC" "$SSH_KEY"
chmod 600 "$SSH_KEY"
trap "rm -f $SSH_KEY" EXIT

wait_for_ssh() {
    echo "[entrypoint] waiting for SSH on ${ONEC_VM_IP}..."
    for i in $(seq 1 30); do
        if ssh -i "$SSH_KEY" -o StrictHostKeyChecking=no -o ConnectTimeout=2 \
               "${ONEC_VM_USER}@${ONEC_VM_IP}" "echo ok" >/dev/null 2>&1; then
            echo "[entrypoint] SSH is reachable"
            return 0
        fi
        sleep 2
    done
    echo "[entrypoint] ERROR: cannot reach SSH on ${ONEC_VM_IP} after 60s"
    return 1
}

mount_sshfs() {
    local remote_path="$1"
    local local_path="$2"
    local label="$3"

    if [ -z "$remote_path" ]; then
        echo "[entrypoint] $label: remote path not set, skipping"
        return 0
    fi

    echo "[entrypoint] mounting $label: ${ONEC_VM_USER}@${ONEC_VM_IP}:${remote_path} -> ${local_path}"
    sshfs "${ONEC_VM_USER}@${ONEC_VM_IP}:${remote_path}" "$local_path" \
        -o IdentityFile="$SSH_KEY" \
        -o StrictHostKeyChecking=no \
        -o UserKnownHostsFile=/dev/null \
        -o allow_other \
        -o ro \
        -o uid=0 \
        -o gid=0 \
        -o umask=0022 \
        -o reconnect \
        -o ServerAliveInterval=15 \
        -o ServerAliveCountMax=3

    if mountpoint -q "$local_path"; then
        echo "[entrypoint] $label mounted OK"
    else
        echo "[entrypoint] ERROR: $label mount failed"
        return 1
    fi
}

wait_for_ssh || exit 1

mount_sshfs "$ONEC_SRVINFO_REMOTE" "/mnt/srvinfo" "srvinfo (event logs)"
mount_sshfs "$ONEC_TECHLOG_REMOTE" "/mnt/techlog" "techlog"

echo "[entrypoint] starting parser..."
exec /app/parser "$@"
