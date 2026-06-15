#!/usr/bin/env bash

redis-server --port 6579 --daemonize yes

PORTS=(6479 6480 6481 6482 6483 6484)

for PORT in "${PORTS[@]}"; do
    mkdir -p "/etc/redis/configs/${PORT}"
    PORT=$PORT envsubst '${PORT}' < redis.conf.template | redis-server - &
done

redis-cli --cluster create 127.0.0.1:6479 127.0.0.1:6480 127.0.0.1:6481 127.0.0.1:6482 127.0.0.1:6483 127.0.0.1:6484 --cluster-replicas 1