#!/usr/bin/env bash
set -euo pipefail

echo "==> Updating package lists"
apt-get update -y
apt-get install -y --no-install-recommends curl ca-certificates wget

echo "==> Installing PostgreSQL"
apt-get install -y postgresql postgresql-contrib

echo "==> Installing Redis (redis-server, redis-cli)"
apt-get install -y redis-server redis-tools

echo "==> Installing MinIO binary"
curl -fsSL "https://dl.min.io/server/minio/release/linux-amd64/minio" \
  -o /usr/local/bin/minio
chmod +x /usr/local/bin/minio

echo "==> Adding Grafana APT repository"
mkdir -p /etc/apt/keyrings
wget -qO /etc/apt/keyrings/grafana.asc https://apt.grafana.com/gpg-full.key
chmod 644 /etc/apt/keyrings/grafana.asc
echo "deb [signed-by=/etc/apt/keyrings/grafana.asc] https://apt.grafana.com stable main" \
  | tee /etc/apt/sources.list.d/grafana.list > /dev/null
apt-get update -y

echo "==> Installing Grafana Alloy"
apt-get install -y alloy

echo "==> Installing Loki"
apt-get install -y loki

echo "==> Installing Grafana"
apt-get install -y grafana

PYROSCOPE_VERSION="2.1.1"
echo "==> Installing Pyroscope ${PYROSCOPE_VERSION}"
curl -fsSL "https://github.com/grafana/pyroscope/releases/download/v${PYROSCOPE_VERSION}/pyroscope_${PYROSCOPE_VERSION}_linux_amd64.deb" \
  -o /tmp/pyroscope.deb
dpkg -i /tmp/pyroscope.deb
rm -f /tmp/pyroscope.deb

echo "==> Installation complete"
psql --version || true
redis-cli --version || true
redis-server --version || true
minio --version || true
alloy --version || true
loki --version || true
grafana server -v || true
pyroscope --version || true