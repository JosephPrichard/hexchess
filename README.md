# Hexagonal Chess
A website to play hexagonal chess online.

Created using Go, Svelte, Postgres, and Redis.

## Run Infrastructure

Starts the redis infrastructure used for caching and message delivery.

`$ docker compose up`

## Build & Execution (Local)

### Compile

Run `build.sh` contained in the root directory. 
This will run `sqlc` to generate the Go DB client, 
    `protoc` to generate the Go serializers, 
    `npm run protogen` to generate the TypeScript serializers,
    and `go build` to generate WASM artifacts for the UI.

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
```

### Run Server

`$ cd hexchess-svc`

`$ go run ./cmd/server/main.go`

### Run UI

`$ cd hexchess-ui`

`$ npm run dev`

## Build & Execution (Prod)

Create an environment variables file in `root`
```
PROXY_PORT=8080
DB_URL=postgres://postgres:<db-password>@host.docker.internal:<db-port>/<db-name>
REDIS_PRIMARY_URL=host.docker.internal:6379
REDIS_PUBSUB_URL=host.docker.internal:6380
ALLOWED_ORIGINS=http://localhost:5173
COOKIE_DOMAIN=localhost
PUBLIC_APP_BASE_URL=http://localhost:8080
PUBLIC_BASE_URL=http://localhost:8080/api
```

This configuration connects to infra running outside the docker container.

`$ docker build . -t hexchess-app:<version>`

`$ docker run -p 8080:8080 -p 6060:6060 --name hexchess-app --env-file .env -d hexchess-app:<version>`

## Environment Variables

`PROXY_PORT` is the port in which the nginx reverse proxy in the docker container will run at, so the port you will hit if you want to use the app running in the docker container.

`SERVER_PORT` is the port where `hexchess-svc` runs at, this is set automatically inside the docker container, but must be set if you are running the server by itself.

`DB_URL` postgres connection url that the server will connect to

`REDIS_PRIMARY_URL` url for redis instance server will use as a cache

`REDIS_PUBSUB_URL` url for redis instance used for message delivery

`ALLOWED_ORIGINS` allowed origins used for CORs, this should be the url the UI is running at

`PUBLIC_APP_BASE_URL` The URL the UI is hosted at

`PUBLIC_BASE_URL` The URL the backend api is hosted at