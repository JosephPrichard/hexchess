# Hexagonal Chess
A website to play hexagonal chess online.

Created using Java, Javascript, Jooby, Handlebars, JQuery, Postgres, and Redis.

## Build and Deployment

### Run Infrastructure

`$ docker compose up`

### Run Server

`$ mvn clean install package`

`$ java -cp target/Hexchess-1.0-SNAPSHOT.jar Main`