# Hexagonal Chess
A website to play hexagonal chess online.

Created using Go, Svelte, Postgres, and Redis.

## Run Infrastructure

Starts the redis infrastructure used for caching and message delivery.

`$ docker compose up`

### Build

Builds generated sources required for development, testing, and deployment.

`make`

Alternatively, run a CI pipeline suitable build that also runs all tests.

`make ci`

## Database Migrations

`cd hexchess-db`

Force Version the Schema (on setup)

`$env:DB_URL="<url>"; $env:MIGRATION_TYPE="<version_number>"; & go run main.go`

Run a Migration (up)

`$env:DB_URL="<url>"; $env:MIGRATION_TYPE="UP"; & go run main.go`

Run a Migration (down)

`$env:DB_URL="<url>"; $env:MIGRATION_TYPE="DOWN"; & go run main.go`

## Execution (Local)

### Env Variables

Create an environment variable file in `hexchess-svc`
```
SERVER_PORT=8081
PPROF_PORT=6060
DB_URL=postgres://postgres:<password>@<db-host>:<db-port>/<db-name>
REDIS_PRIMARY_URL=localhost:6379
REDIS_PUBSUB_URL=localhost:6380
ALLOWED_ORIGINS=http://localhost:5173
COOKIE_DOMAIN=localhost
AWS_SECRET_ID=test
AWS_SECRET_KEY=test
AWS_DEFAULT_REGION=us-east-1
AWS_ENDPOINT=http://localhost:4566
```

### Run Server

`$ cd hexchess-svc`

`$ go run cmd/server/main.go`

### Run UI

`$ cd hexchess-ui`

`$ npm run dev`

## Build & Execution (Prod)

Create an environment variable file in `root`
```
SERVER_PORT=8080
PPROF_PORT=6060
DB_URL=postgres://postgres:<db-password>@host.docker.internal:<db-port>/<db-name>
REDIS_PRIMARY_URL=host.docker.internal:6379
REDIS_PUBSUB_URL=host.docker.internal:6380
ALLOWED_ORIGINS=<ui application hostname in route53>
COOKIE_DOMAIN=localhost
AWS_SECRET_ID=<automatically set in aws>
AWS_SECRET_KEY=<automatically set in aws>
AWS_DEFAULT_REGION=us-east-1
AWS_ENDPOINT=
```

This configuration connects to infra running outside the docker container.

`$ docker build . -t hexchess-app:<version>`

`$ docker run -p 8080:8080 -p 6060:6060 --name hexchess-app --env-file .env -d hexchess-app:<version>`

## Environment Variables

`SERVER_PORT` is the port where `hexchess-svc` runs at, this is must be the same as what the ALB is configured to direct traffic to.

`DB_URL` postgres connection url that the server will connect to

`REDIS_PRIMARY_URL` url for redis instance server will use as a cache

`REDIS_PUBSUB_URL` url for redis instance used for message delivery

`ALLOWED_ORIGINS` allowed origins used for CORs, this should be the url the UI is running at

`AWS_SECRET_ID` Standard AWS credentials environment variable.

`AWS_SECRET_KEY` Standard AWS credentials environment variable.

`AWS_DEFAULT_REGION` The region the AWS infrastructure resources (only S3 as of right now) are in.

`AWS_ENDPOINT` The AWS endpoint to point the S3 Client to, this can be set to localstack for testing but should be left empty for prod (points to real AWS endpoint by default).