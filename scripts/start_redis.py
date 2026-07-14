#!/usr/bin/env python3
import os
import subprocess
import tempfile
from pathlib import Path
from string import Template

REDIS_PORTS = [6479, 6480, 6481, 6482, 6483, 6484]

print(f"stopping existing redis nodes on ports: {' '.join(str(p) for p in REDIS_PORTS)}")
for port in REDIS_PORTS:
    subprocess.run(["fuser", "-k", f"{port}/tcp"])

print("starting redis cluster")

# redis configs
REDIS_CONFIGS_PATH = "/workspace/redis/configs"

# starts a classic 3 master 3 slave redis cluster for system state. each individual node is configured to be cluster aware
print(f'starting redis cluster nodes on ports={REDIS_PORTS}')
for port in REDIS_PORTS:
    os.makedirs(f"{REDIS_CONFIGS_PATH}/{port}", exist_ok=True)
    subprocess.Popen(
        f"PORT={port} envsubst '${{PORT}}' < ./redis.conf.template | redis-server -",
        shell=True,
    )

print(f'starting redis cluster on ports={REDIS_PORTS}')
subprocess.run(
    [
        "redis-cli",
        "--cluster", "create",
        "127.0.0.1:6479", "127.0.0.1:6480", "127.0.0.1:6481",
        "127.0.0.1:6482", "127.0.0.1:6483", "127.0.0.1:6484",
        "--cluster-yes",
        "--cluster-replicas", "1",
    ]
)