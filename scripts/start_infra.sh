#!/usr/bin/env bash

# starts minio as a mock backend for s3, credentials here match the test credentials in the backend
export MINIO_ROOT_USER=testing
export MINIO_ROOT_PASSWORD=testing

minio server /mnt/data --console-address ":9101" --address ":9100" &

# starts a single redis node for pubsub (it cannot be part of the cluster)
redis-server --port 6579 --daemonize yes

# starts a classic 3 master 3 slave redis cluster for system state. each individual node is configured to be cluster aware
REDIS_PORTS=(6479 6480 6481 6482 6483 6484)

for PORT in "${REDIS_PORTS[@]}"; do
    mkdir -p "/etc/redis/configs/${PORT}"
    PORT=$PORT envsubst '${PORT}' < redis.conf.template | redis-server - &
done

redis-cli --cluster create 127.0.0.1:6479 127.0.0.1:6480 127.0.0.1:6481 127.0.0.1:6482 127.0.0.1:6483 127.0.0.1:6484 --cluster-replicas 1