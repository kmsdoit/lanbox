#!/bin/sh
# systemd 모드에서 PID 1 환경변수를 읽어 SSH 키와 비밀번호를 넣는다.
set -eu

if [ ! -s /etc/ssh/ssh_host_ed25519_key ]; then
  ssh-keygen -A
fi

if [ -r /proc/1/environ ]; then
  envfile=$(mktemp)
  tr '\0' '\n' < /proc/1/environ | grep '^LANBOX_' > "${envfile}" || true
  # shellcheck disable=SC1090
  set -a
  . "${envfile}"
  set +a
  rm -f "${envfile}"
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
