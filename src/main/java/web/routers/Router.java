package web.routers;

import io.jooby.Jooby;
import io.jooby.jackson.JacksonModule;
import web.State;

import java.time.Duration;

import static utils.Globals.LOGGER;

public class Router extends Jooby {

    public Router(State state) {
        install(new JacksonModule());

        error((ctx, cause, statusCode) -> {
            ctx.setResponseCode(statusCode);
            if (statusCode.value() == 500) {
                // internal server errors contain stack traces, so log them out to server but do not end to client.js
                LOGGER.error("Error: {} ", statusCode, cause);
                ctx.send("Internal Server Error");
            } else {
                // non 500 errors contain clear messages that can be spit out as strings to both server logs and the client.js
                String message = "Error: " + statusCode + ", " + cause.getMessage();
                LOGGER.error(message);
                ctx.send(cause.getMessage());
            }
        });

        assets("/static/*", "static").setMaxAge(Duration.ofHours(1));

        mount(new FormRouter(state));
        mount(new PageRouter(state).initStatics());
        mount(new PartialsRouter(state));
        mount(new GameRouter(state));
    }
}
