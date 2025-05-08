package web.controllers;

import io.jooby.Jooby;
import io.jooby.ServerOptions;
import io.jooby.jackson.JacksonModule;
import web.State;

import java.time.Duration;

public class AppController extends Jooby {

    public AppController(int port, State state) {
        setServerOptions(new ServerOptions().setPort(port));

        install(new JacksonModule());

        mount(new FormController(state));
        mount(new EventController(state));
        mount(new ViewController(state));
        mount(new WsController(state));
    }
}
