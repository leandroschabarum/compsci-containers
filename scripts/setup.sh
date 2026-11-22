#!/bin/sh
# Prepara a VM Alpine: baixa e compila as imagens e registra o que roda no boot.
# A stack sobe depois, à mão, com podman-compose up -d. Rodar como root: sh scripts/setup.sh
set -eu
cd "$(dirname "$0")/.."
ROOT=$(pwd)

podman-compose pull
podman-compose build

# Na inicialização da VM: sobe os contêineres com restart=always (o Alpine
# não tem o podman-restart.service do systemd) e o vigia de healthcheck.
cat > /etc/local.d/shortener.start <<EOF
#!/bin/sh
podman start --all --filter restart-policy=always
pkill -f healthwatch.sh
nohup sh $ROOT/scripts/healthwatch.sh >>/var/log/healthwatch.log 2>&1 &
EOF
chmod +x /etc/local.d/shortener.start
rc-update add local default || true
/etc/local.d/shortener.start

echo
echo "Imagens prontas e healthwatch rodando. Suba a stack com: podman-compose up -d"
echo "API: http://<ip-da-vm>:8080  Grafana: http://<ip-da-vm>:3000  Prometheus: http://<ip-da-vm>:9090"
echo "Grafana abre sem login. Usuário admin: admin / admin."
