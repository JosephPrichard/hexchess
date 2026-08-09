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

Runs the backend functional testing and non-functional testing suites.

`make functional-test`

`make perf-test`

TODO
`make e2e-test`

### Env Variables

Create an environment variable file in `backend`

Sample:
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

### Database Migrations

Run a Migration (up)

`$ cd migrations && go run main.go`

### Run Servers

`$ cd backend`

`$ go run cmd/server/api/main.go`

`$ go run cmd/server/consumers/main.go`

### Run UI

`$ cd frontend`

`$ npm run dev`

## Deployment

Deployment to AWS infrastructure is done using Terraform scripts. There's one terraform script for each component: `infra`, `consumer`, `migrator`, `api`, and `frontend`.

Pipeline is trunk style, from `main` only. It will deploy all apps to all environments (only UAT for now).