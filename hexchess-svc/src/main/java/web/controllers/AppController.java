package web.controllers;

import io.jooby.Jooby;
import io.jooby.ServerOptions;
import io.jooby.handler.Cors;
import io.jooby.handler.CorsHandler;
import io.jooby.jackson.JacksonModule;
import web.State;

import java.time.Duration;
import java.util.List;

public class AppController extends Jooby {

    public AppController(int port, List<String> allowedOrigins, State state) {
        setServerOptions(new ServerOptions().setPort(port));

        use(new CorsHandler(new Cors().setOrigin(allowedOrigins)));

        install(new JacksonModule());

        mount(new FormController(state));
        mount(new EventController(state));
        mount(new ViewController(state));
        mount(new WsController(state));
    }
}
