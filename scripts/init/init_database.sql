-- INIT script to create ROLES/USERS after the DB is created (must be done from root user)

-- ROLES: users db_readwrite
CREATE USER db_readwrite WITH LOGIN;

GRANT rds_superuser TO dbadmin; -- matches master user configured in terraform file
GRANT rds_superuser TO db_readwrite; -- required to create extensions

GRANT rds_iam TO db_readwrite;

GRANT CONNECT ON DATABASE hexchess TO db_readwrite;

ALTER USER db_readwrite CREATEDB;

-- ROLE: db_readwrite: full access to manage the database

-- Grant usage on the schema
GRANT ALL PRIVILEGES ON SCHEMA public TO db_readwrite;

-- Grant privileges on existing tables, sequences,and functions
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO db_readwrite;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO db_readwrite;
GRANT ALL PRIVILEGES ON ALL FUNCTIONS IN SCHEMA public TO db_readwrite;

-- Grant privileges on futures tables, sequences, and functions
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL PRIVILEGES ON TABLES TO db_readwrite;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL PRIVILEGES ON SEQUENCES TO db_readwrite;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL PRIVILEGES ON FUNCTIONS TO db_readwrite;

GRANT CREATE ON SCHEMA public TO db_readwrite;