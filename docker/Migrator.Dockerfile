FROM golang:1.26-alpine

WORKDIR /sources

# ---- Dependency Layer ----
COPY migrations/go.mod migrations/go.sum ./

RUN go mod download

# ---- Compile Layer ----
COPY migrations ./

RUN CGO_ENABLED=0 go build -trimpath -ldflags=-s -o ./app .

CMD ["./app"]