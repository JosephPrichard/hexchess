package web.controllers;

import io.jooby.Jooby;
import io.jooby.jackson.JacksonModule;
import web.State;

import java.time.Duration;

public class AppController extends Jooby {

    public AppController(State state) {
        install(new JacksonModule());

        mount(new StaticsController());
        mount(new FormController(state));
        mount(new EventController(state));
        mount(new PageController(state).initStatics());
        mount(new PartialsController(state));
        mount(new WebsocketController(state));
    }
}
