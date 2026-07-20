# ---- Build Stage ----
FROM golang:1.26-alpine AS builder
RUN apk add --no-cache make
RUN apk add --no-cache bash
RUN apk add --no-cache protobuf protobuf-dev

WORKDIR /sources

# ---- Dependency Layer ----
COPY backend/install.sh backend/go.mod backend/go.sum backend/

RUN ./backend/install.sh
RUN cd backend && go mod download

# ---- Codegen Layer ----
COPY Makefile .
COPY contracts contracts/
COPY backend backend/

RUN make backend --always-make

# ---- Compile Layer ----
ARG CMD_KIND=api
RUN cd backend && CGO_ENABLED=0 go build -trimpath -ldflags=-s -o /bin/app ./cmd/server/${CMD_KIND}

# ---- Runtime Stage ----
FROM alpine:latest AS runner

# Copy binary
COPY --from=builder /bin/app ./app

EXPOSE 8080
CMD ["./app"]