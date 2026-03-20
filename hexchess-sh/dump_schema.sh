#!/bin/bash
set -e

PGPASSWORD="${PASSWORD:?PASSWORD env variable is required}" \
  pg_dump --schema-only --no-owner --no-acl \
  -h localhost \
  -U postgres \
  -d hexchessdb \
  -f ../hexchess-svc/db/schema.sql

echo "Schema dumped to ../hexchess-svc/db/schema.sql"