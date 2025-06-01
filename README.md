# Hexagonal Chess
A website to play hexagonal chess online.

Created using Java, Svelte, Jooby, Postgres, and Redis.

## Build and Deployment

### Install Dependencies

`$ git clone https://github.com/google/flatbuffers.git`
`$ sudo apt update`
`$ sudo apt install cmake`
``

### Run Infrastructure

`$ docker compose up`

### Run Server

`$ mvn clean install package`

`$ java -cp target/Hexchess-1.0-SNAPSHOT.jar Main`