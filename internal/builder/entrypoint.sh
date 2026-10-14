#!/bin/sh
# light 모드 PID 1 다음 프로세스. Docker --init(tini)가 이 스크립트를 실행한다.
set -eu

if [ ! -s /etc/ssh/ssh_host_ed25519_key ]; then
  ssh-keygen -A
fi

user="${LANBOX_USER:-lanbox}"
home_dir="/home/${user}"
if [ -n "${LANBOX_SSH_PUBKEY:-}" ]; then
  mkdir -p "${home_dir}/.ssh"
  printf '%s\n' "${LANBOX_SSH_PUBKEY}" > "${home_dir}/.ssh/authorized_keys"
  chown -R "${user}:${user}" "${home_dir}/.ssh" 2>/dev/null || chown -R "${user}" "${home_dir}/.ssh"
  chmod 700 "${home_dir}/.ssh"
  chmod 600 "${home_dir}/.ssh/authorized_keys"
fi
if [ -n "${LANBOX_PASSWORD:-}" ]; then
  printf '%s:%s\n' "${user}" "${LANBOX_PASSWORD}" | chpasswd
fi

exec /usr/sbin/sshd -D -e
