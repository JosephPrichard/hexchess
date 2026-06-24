#!/usr/bin/env bash

REDIS_PORTS=(6479 6480 6481 6482 6483 6484)
REDIS_PUBSUB_PORT=6579
MINIO_PORT=9100
ALLOY_PORT=12345

PORTS=("${REDIS_PORTS[@]}" "${REDIS_PUBSUB_PORT}" "${MINIO_PORT}" "${ALLOY_PORT}")

echo "stopping existing infrastructure on ports: ${PORTS[*]}"

for PORT in "${PORTS[@]}"; do
    fuser -k ${PORT}/tcp
done

# a script for starting infrastructure needed for testing and development. this includes redis, minio, and alloy.
echo "starting infrastructure"

# starts alloy, which should be configured to pull pprof data from the application
CONFIG_FILE="./config.alloy"
LOG_FILE="$HOME/alloy.log"

alloy run "$CONFIG_FILE" --storage.path="$HOME/.alloy-data" > "$LOG_FILE" 2>&1 &
echo "started alloy"

# starts minio as a mock backend for s3, credentials here match the test credentials in the backend
export MINIO_ROOT_USER=test-username
export MINIO_ROOT_PASSWORD=test-password

minio server /mnt/data --console-address ":9101" --address ":${MINIO_PORT}" &
echo "started minio"

# starts a single redis node for pubsub (it cannot be part of the cluster)
redis-server --port ${REDIS_PUBSUB_PORT} - > /dev/null 2>&1 &
echo "started redis node"

# starts a classic 3 master 3 slave redis cluster for system state. each individual node is configured to be cluster aware
REDIS_CONFIGS_PATH="/etc/redis/configs"

for PORT in "${REDIS_PORTS[@]}"; do
    mkdir -p "${REDIS_CONFIGS_PATH}/${PORT}"
    PORT=$PORT envsubst '${PORT}' < redis.conf.template | redis-server - > /dev/null 2>&1 &
done
echo "started redis cluster nodes"

redis-cli --cluster create 127.0.0.1:6479 127.0.0.1:6480 127.0.0.1:6481 127.0.0.1:6482 127.0.0.1:6483 127.0.0.1:6484 --cluster-replicas 1
echo "started redis cluster"

# starts standard pyroscope server (assumed to be on port 4040, referenced in alloy config)
systemctl start pyroscope
echo "started pyroscope"

# starts standard loki (referenced in app env vars)
systemctl start loki
echo "started loki"

# starts standard grafana UI (setup pyroscope and loki configs from UI)
systemctl start grafana-server
echo "started grafana"

# starts standard postgres (configure the connection to this server in env vars of the app)
systemctl start postgresql
echo "started postgres"