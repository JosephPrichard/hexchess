# dump_schema.ps1

if (-not $env:PGPASSWORD) {
    Write-Error "PGPASSWORD env variable is required"
    exit 1
}

& "C:\Program Files\PostgreSQL\17\bin\pg_dump.exe" `
    -s `
    --no-owner `
    --no-privileges `
    -h localhost `
    -p 5433 `
    -U postgres `
    -d hexachess2 | `
ForEach-Object {
    $_ -replace "SELECT pg_catalog.set_config\('search_path', '', false\);", "SELECT pg_catalog.set_config('search_path', 'public', false);"
} | Set-Content ../backend/db/schema.sql

if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}

Write-Host "Schema dumped to ../backend/db/schema.sql"