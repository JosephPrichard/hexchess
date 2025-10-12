# Hexagonal Chess
A website to play hexagonal chess online.

Created using Go, Svelte, Postgres, and Redis.

Currently, in the process of a rewrite from Java -> Go.

## Build and Deployment

### Compile Server

`$ cd hexchess-svc`

`$ sqlc generate`

`$ protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative pb/messages.proto`

### Compile UI

`$ cd hexchess-ui`

`$ npm run protogen`

### Run Infrastructure

`$ docker compose up`

### Set Environment Variables

Create an environment variables file in the `resources` folder of `hexchess-svc`
```
APP_PORT=8081
ELASTICSEARCH_USERNAME=elasticname
ELASTICSEARCH_PASSWORD=elasticsearch-password
DB_PASSWORD=db-password
DB_URL=jdbc:postgresql://localhost:5432/hexachess
DB_USER=postgres
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PUBSUB_HOST=localhost
REDIS_PUBSUB_PORT=6380
ALLOWED_ORIGINS=http://localhost:5173
COOKIE_DOMAIN=localhost
```

### Run Server Locally

`$ cd hexchess-svc`

`$ go run ./cmd/server/main.go`

### Build Docker Image

`$ cd hexchess-svc`

`$ docker build -t hexchess-svc:<version>`