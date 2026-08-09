#!/usr/bin/env fish
# Note(Joseph): Must be run from inside this directory, not parent
set -x

set REDIS_PORTS 6479 6480 6481 6482 6483 6484
set REDIS_PUBSUB_PORT 6579
set MINIO_PORT 9100
set LOKI_HTTP_PORT 3100
set LOKI_GRPC_PORT 9096
set PYRO_HTTP_PORT 4041
set PYRO_GRPC_PORT 9098
set ALLOY_PORT 12345

set PORTS $REDIS_PORTS $REDIS_PUBSUB_PORT $MINIO_PORT $ALLOY_PORT $LOKI_HTTP_PORT $LOKI_GRPC_PORT $PYRO_HTTP_PORT $PYRO_GRPC_PORT

echo "==> stopping existing infrastructure on ports: $PORTS"
for PORT in $PORTS
    fuser -k "$PORT/tcp"
    echo "==> stopped infrastructure on port: $PORT"
end

systemctl stop postgresql
systemctl stop grafana-server

echo "==> starting infrastructure"
set WORKSPACE "$PWD/data"
#set WORKSPACE "/etc/workspace/hexchess/data"
mkdir -p "$WORKSPACE"

# starts minio as a mock backend for s3, credentials here match the test credentials in the backend
set -gx MINIO_ROOT_USER test-username
set -gx MINIO_ROOT_PASSWORD test-password
set MINIO_STORAGE_PATH "$WORKSPACE/minio"
minio server "$MINIO_STORAGE_PATH" --console-address ":9101" --address ":$MINIO_PORT" &
echo "==> started minio"

# starts a single redis node for pubsub (it cannot be part of the cluster)
PORT=$REDIS_PUBSUB_PORT envsubst '${PORT}' < ./configs/redis.single.conf.template | redis-server - &
echo "==> started redis pubsub"

# starts a classic 3 master 3 slave redis cluster for system state. each individual node is configured to be cluster aware
for PORT in $REDIS_PORTS
    mkdir -p "$WORKSPACE/redis/configs/$PORT"
    PORT=$PORT WORKSPACE=$WORKSPACE envsubst '${PORT} ${WORKSPACE}' < ./configs/redis.conf.template | redis-server - &
end
echo "==> started redis cluster nodes"

set -g NODES
for PORT in $REDIS_PORTS
    set -g --append NODES "127.0.0.1:$PORT"
end

# starts alloy with a custom config
set ALLOY_CONFIG_FILE "./configs/config.alloy"
set ALLOY_LOG_FILE "$WORKSPACE/alloy.log"
set ALLOY_STORAGE_PATH "$WORKSPACE/.alloy-data"
alloy run "$ALLOY_CONFIG_FILE" --storage.path="$ALLOY_STORAGE_PATH" > "$ALLOY_LOG_FILE" 2>&1 &
echo "==> started alloy"

# starts loki with custom config
set LOKI_DATA_DIR "$WORKSPACE/loki"
set LOKI_CONFIG_FILE "./configs/loki.yaml"
sudo mkdir -p "$LOKI_DATA_DIR"
loki -config.file="$LOKI_CONFIG_FILE" -config.expand-env=true &
echo "==> started loki"

# starts pyroscope server with custom config
set PYRO_CONFIG_FILE "./configs/pyroscope.yaml"
pyroscope -config.file="$PYRO_CONFIG_FILE" &
echo "==> started pyroscope"

redis-cli --cluster create 127.0.0.1:6479 127.0.0.1:6480 127.0.0.1:6481 127.0.0.1:6482 127.0.0.1:6483 127.0.0.1:6484 --cluster-replicas 1 --cluster-yes
echo "==> started redis cluster with nodes $NODES"

# starts standard postgres
systemctl start postgresql
echo "==> started postgres"

# starts standard grafana UI
systemctl start grafana-server
echo "==> started grafana"