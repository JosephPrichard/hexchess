CREATE DATABASE hexchess;
CREATE DATABASE metrics;

CREATE USER db_readwrite WITH LOGIN;
CREATE USER db_migrator  WITH LOGIN;

GRANT rds_iam TO db_readwrite;
GRANT rds_iam TO db_migrator;

ALTER USER db_migrator CREATEDB;

GRANT CONNECT ON DATABASE hexchess TO db_readwrite, db_migrator;
GRANT CONNECT ON DATABASE metrics  TO db_readwrite, db_migrator;

SELECT datname AS database_name
FROM pg_database
WHERE datistemplate = false
ORDER BY datname;

SELECT usename FROM pg_catalog.pg_user;