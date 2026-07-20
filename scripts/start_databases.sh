#!/usr/bin/env bash

set -x

REDIS_PORTS=(6479 6480 6481 6482 6483 6484)
REDIS_PUBSUB_PORT=6579
MINIO_PORT=9100
LOKI_HTTP_PORT=3100
LOKI_GRPC_PORT=9096
PYRO_HTTP_PORT=4041
PYRO_GRPC_PORT=9098
ALLOY_PORT=12345

PORTS=("${REDIS_PORTS[@]}" "${REDIS_PUBSUB_PORT}" "${MINIO_PORT}" "${ALLOY_PORT}" "${LOKI_HTTP_PORT}" "${LOKI_GRPC_PORT}" "${PYRO_HTTP_PORT}" "${PYRO_GRPC_PORT}")

echo "==> stopping existing infrastructure on ports: ${PORTS[*]}"
systemctl stop postgresql
systemctl stop grafana-server

for PORT in "${PORTS[@]}"; do
    fuser -k "${PORT}"/tcp
done

echo "==> starting infrastructure"
WORKSPACE="$PWD/data"
#WORKSPACE="/etc/workspace/hexchess/data"
mkdir -p "${WORKSPACE}"

# starts standard postgres
systemctl start postgresql
echo "==> started postgres"

# starts minio as a mock backend for s3, credentials here match the test credentials in the backend
export MINIO_ROOT_USER=test-username
export MINIO_ROOT_PASSWORD=test-password

MINIO_STORAGE_PATH="${WORKSPACE}/minio"
minio server "${MINIO_STORAGE_PATH}" --console-address ":9101" --address ":${MINIO_PORT}" &
echo "==> started minio"

# starts a single redis node for pubsub (it cannot be part of the cluster)
# shellcheck disable=SC2016
PORT=${REDIS_PUBSUB_PORT} envsubst '${PORT}' < ./configs/redis.single.conf.template | redis-server - &
echo "==> started redis pubsub"

# starts a classic 3 master 3 slave redis cluster for system state. each individual node is configured to be cluster aware
for PORT in "${REDIS_PORTS[@]}"; do
    mkdir -p "${WORKSPACE}/redis/configs/${PORT}"
    # shellcheck disable=SC2016
    PORT=$PORT WORKSPACE=$WORKSPACE envsubst '${PORT} ${WORKSPACE}' < ./configs/redis.conf.template | redis-server - &
done
echo "==> started redis cluster nodes"

NODES=()
for PORT in "${REDIS_PORTS[@]}"; do
  NODES+=("127.0.0.1:${PORT}")
done

# stats alloy with a custom config
ALLOY_CONFIG_FILE="./configs/config.alloy"
ALLOY_LOG_FILE="${WORKSPACE}/alloy.log"
ALLOY_STORAGE_PATH="${WORKSPACE}/.alloy-data"

alloy run "$ALLOY_CONFIG_FILE" --storage.path="$ALLOY_STORAGE_PATH" > "$ALLOY_LOG_FILE" 2>&1 &
echo "==> started alloy"

# starts standard grafana UI
systemctl start grafana-server
echo "==> started grafana"

# starts loki with custom config
LOKI_DATA_DIR="${WORKSPACE}/loki"
LOKI_CONFIG_FILE="./configs/loki.yaml"

sudo mkdir -p "${LOKI_DATA_DIR}"

loki -config.file="${LOKI_CONFIG_FILE}" -config.expand-env=true &
echo "==> started loki"

# starts pyroscope server with custom config
PYRO_CONFIG_FILE="./configs/pyroscope.yaml"

pyroscope -config.file="${PYRO_CONFIG_FILE}" &
echo "==> started pyroscope"

redis-cli --cluster create 127.0.0.1:6479 127.0.0.1:6480 127.0.0.1:6481 127.0.0.1:6482 127.0.0.1:6483 127.0.0.1:6484 --cluster-replicas 1 --cluster-yes
echo "==> started redis cluster with nodes ${NODES[*]}"