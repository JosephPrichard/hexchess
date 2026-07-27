# Hexagonal Chess
A website to play hexagonal chess online. It's primarily a way for me to explore ideas related to backend infrastructure, software architecture, performance optimization, and functional testing.

Languages: Go, TypeScript, Svelte, Protoc

Build: Docker, Terraform

Infrastructure: AWS, Postgres, and Redis

## Development

### Environment

Additionally, provides a shell script to start infrastructure that the app connects to.

`cd scripts && sudo ./start_infra.sh`

### Build

Project uses Makefile for shell script automation

Builds generated sources required for development, testing, and scripting.

`make`

Runs the functional testing and non-functional testing suite.

`make test`

`make perf-test`

### Database Migrations

`export GOOSE_DBSTRING=<database-url>`

`export GOOSE_DBDRIVER=postgres`

Run a Migration (up)

`cd migrations && goose up`

Run a Migration (down)

`cd migrations && goose down`

### Env Variables

Create an environment variable file in `backend`
```
SERVER_PORT=8080
DB_URL=postgresql://postgres:<password>@localhost:5432/hexchess
REDIS_SOR_NODES=localhost:6479,localhost:6480,localhost:6481,localhost:6482,localhost:6483,localhost:6484
REDIS_PUBSUB_NODE=localhost:6579
ACTIVE_PROFILE=local
AWS_REGION=us-east-1
AWS_ENDPOINT=http://localhost:9100
AWS_USERNAME=test-username
AWS_PASSWORD=test-password
ALLOWED_ORIGINS=http://localhost:5173
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:3100
```

`SERVER_PORT` The port where `backend` runs at, this is must be the same as what the ALB is configured to direct traffic to.

`DB_URL` Postgres connection url that the server will connect to.

`REDIS_SOR_NODES` Node URIs for redis instance server will use for caching and system of record

`REDIS_PUBSUB_NODE` URI for redis instance used for message delivery

`ALLOWED_ORIGINS` Allowed origins used for CORs, this should be the URI the UI is running at.

`OTEL_EXPORTER_OTLP_ENDPOINT` Allows the app to forward logs to an oltp compatible server. Setup to your loki endpoint or leave blank to turn off oltp logging.

`PROFILE` Decides the profile (e.g local, test, prod) that will be used to initialize the app. When local is flipped on, AWS authentication is turned off.

`AWS_DEFAULT_REGION` The region the AWS infrastructure resources are in. Ideally us-east-1 because multi region configs are not supported yet.

`AWS_ENDPOINT` The AWS endpoint to point the S3 Client to, this can be set to minio for testing but should be left empty for prod (points to real AWS endpoint by default).

`AWS_SECRET_ID` Standard AWS credentials environment variable.

`AWS_SECRET_KEY` Standard AWS credentials environment variable.

### Run Servers

`$ cd backend`

`$ go run cmd/server/api/main.go`

`$ go run cmd/server/consumers/main.go`

### Run UI

`$ cd frontend`

`$ npm run dev`