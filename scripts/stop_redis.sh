#!/usr/bin/env bash

PORTS=(6479 6480 6481 6482 6483 6484 6579)

for PORT in "${PORTS[@]}"; do
    sudo fuser -k ${PORT}/tcp
done