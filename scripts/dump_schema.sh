#!/usr/bin/env bash
set -e

pg_dump \
    -s \
    --no-owner \
    --no-privileges \
    -h localhost \
    -p 5432 \
    -U postgres \
    -d hexchess | \
sed -e "s/SELECT pg_catalog.set_config('search_path', '', false);/SELECT pg_catalog.set_config('search_path', 'public', false);/" \
    -e '/^\\restrict/d' \
    -e '/^\\unrestrict/d' \
    > ../backend/db/schema.sql