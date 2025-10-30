FROM golang:1.24 AS go-builder
WORKDIR /app/go
COPY hexchess-svc/ ./
RUN go mod download && cd cmd/server && go build -o /go-server .

FROM node:20 AS node-builder
WORKDIR /app/node
COPY hexchess-ui/ ./
RUN npm install && npm run build

FROM node:20

RUN apt-get update && apt-get install -y nginx gettext-base && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=go-builder /go-server /app/go-server
COPY --from=node-builder /app/node /app/node

COPY nginx.conf /etc/nginx/nginx.conf

EXPOSE 8080

CMD SERVER_PORT=8081 /app/go-server & \
    PORT=5173 node --trace-warnings /app/node/build/index.js & \
    nginx -g "daemon off;"
