#!/usr/bin/env bash

# stops any infrastructure started by the start_infra.sh script
PORTS=(6479 6480 6481 6482 6483 6484 6579 9100 9101)

for PORT in "${PORTS[@]}"; do
    sudo fuser -k ${PORT}/tcp
done