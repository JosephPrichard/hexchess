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

`cd database`

Run a Migration (up)

`$env:GOOSE_DBSTRING="<url>"; goose up`

Run a Migration (down)

`$env:GOOSE_DBSTRING="<url>"; goose down`

## Execution (Local)

### Env Variables

Create an environment variable file in `backend`
```
SERVER_PORT=8081
DB_URL=postgres://postgres:<password>@<db-host>:<db-port>/<db-name>
REDIS_SOR_NODES=localhost:6379
REDIS_PUBSUB_NODE=localhost:6579
IS_LOCAL_S3=true
AWS_DEFAULT_REGION=us-east-2
ALLOWED_ORIGINS=http://localhost:5173
```

### Run Server

`$ cd backend`

`$ go run cmd/server/main.go`

### Run UI

`$ cd frontend`

`$ npm run dev`

## Build & Execution (Prod)

Create an environment variable file in `root`
```
SERVER_PORT=8080
PPROF_PORT=6060
DB_URL=postgres://postgres:<db-password>@host.docker.internal:<db-port>/<db-name>
REDIS_PRIMARY_URL=host.docker.internal:6379
REDIS_PUBSUB_URL=host.docker.internal:6380
ALLOWED_ORIGINS=<hostname>
COOKIE_DOMAIN=localhost
AWS_SECRET_ID=<aws-credentials>
AWS_SECRET_KEY=<aws-credentials>
AWS_DEFAULT_REGION=us-east-1
AWS_ENDPOINT=
```

This configuration connects to infra running outside the docker container.

`$ docker build . -t hexchess-app:<version>`

`$ docker run -p 8080:8080 -p 6060:6060 --name hexchess-app --env-file .env -d hexchess-app:<version>`

## Environment Variables

`SERVER_PORT` is the port where `backend` runs at, this is must be the same as what the ALB is configured to direct traffic to.

`DB_URL` postgres connection url that the server will connect to

`REDIS_SOR_NODES` node urls for redis instance server will use for caching and system of record

`REDIS_PUBSUB_NODE` url for redis instance used for message delivery

`ALLOWED_ORIGINS` allowed origins used for CORs, this should be the url the UI is running at

`AWS_SECRET_ID` Standard AWS credentials environment variable.

`AWS_SECRET_KEY` Standard AWS credentials environment variable.

`IS_LOCAL` Decides if we should turn on local mocks for downstream services such as AWS

`AWS_DEFAULT_REGION` The region the AWS infrastructure resources (only S3 as of right now) are in.

`AWS_ENDPOINT` The AWS endpoint to point the S3 Client to, this can be set to localstack for testing but should be left empty for prod (points to real AWS endpoint by default).