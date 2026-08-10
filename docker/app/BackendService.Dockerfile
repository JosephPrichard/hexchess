# ---- Build Stage ----
FROM golang:1.26-alpine AS builder

WORKDIR /sources

# ---- Dependency Layer ----
COPY backend/go.mod backend/go.sum backend/

RUN cd backend && go mod download

# ---- Codegen Layer ----
COPY contracts contracts/
COPY backend backend/

# ---- Compile Layer ----
ARG SERVICE=api
RUN cd backend && CGO_ENABLED=0 go build -trimpath -ldflags=-s -o /bin/app ./cmd/server/${SERVICE}

# ---- Runtime Stage ----
FROM alpine:latest AS runner

# Copy binary
COPY --from=builder /bin/app ./app

EXPOSE 8080

# ENTRYPOINT allows CLI arguments to be injected from the caller
ENTRYPOINT ["./app"]