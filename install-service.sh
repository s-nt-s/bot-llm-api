#!/usr/bin/env bash

set -euo pipefail

service_name="${1:-bot-api}"
project_dir="${2:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
service_user="${3:-${SUDO_USER:-${USER}}}"
unit_path="/etc/systemd/system/${service_name}.service"

if [[ ${EUID} -ne 0 ]]; then
	echo "Este script debe ejecutarse como root (por ejemplo: sudo $0)." >&2
	exit 1
fi

if [[ ! ${service_name} =~ ^[a-zA-Z0-9@_.-]+$ ]]; then
	echo "Nombre de servicio invalido: ${service_name}" >&2
	exit 1
fi

if [[ ! -f "${project_dir}/.env" ]]; then
	echo "No existe ${project_dir}/.env" >&2
	exit 1
fi

if ! id "${service_user}" >/dev/null 2>&1; then
	echo "El usuario no existe: ${service_user}" >&2
	exit 1
fi

project_dir="$(cd "${project_dir}" && pwd)"
go_path="$(sudo -u "${service_user}" -H bash -lc 'command -v go' 2>/dev/null || true)"
if [[ -z ${go_path} ]]; then
	echo "No se encontró el ejecutable go en el entorno de ${service_user}" >&2
	exit 1
fi

cat > "${unit_path}" <<EOF
[Unit]
Description=bot-api
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${service_user}
WorkingDirectory=${project_dir}
EnvironmentFile=${project_dir}/.env
ExecStart=${go_path} run ${project_dir}/
Restart=on-failure
RestartSec=5s
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF

chmod 0644 "${unit_path}"
systemctl daemon-reload
systemctl enable --now "${service_name}.service"

echo "Servicio instalado y arrancado: ${service_name}.service"
echo "Estado: systemctl status ${service_name}.service"