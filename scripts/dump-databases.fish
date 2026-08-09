#!/usr/bin/env fish

function dump_schema
    set -l db_name $argv[1]
    pg_dump \
        -s \
        --no-owner \
        --no-privileges \
        -h localhost \
        -p 5432 \
        -U postgres \
        -d "$db_name" | \
    sed -e "s/SELECT pg_catalog.set_config('search_path', '', false);/SELECT pg_catalog.set_config('search_path', 'public', false);/" \
        -e '/^\\restrict/d' \
        -e '/^\\unrestrict/d' \
        > "../backend/db/schema.sql"
end

dump_schema "hexchess"