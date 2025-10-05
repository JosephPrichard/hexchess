package web;

import io.jooby.*;
import io.jooby.handler.Cors;
import io.jooby.handler.CorsHandler;
import io.jooby.jackson.JacksonModule;
import models.views.ServiceView;

import java.util.List;

import static utils.Globals.TP;
import static utils.Globals.LOG;
import static web.WebConstants.ERROR_UNKNOWN;

public class AppController extends Jooby {

    public AppController(int port, List<String> allowedOrigins, State state) {
        setServerOptions(new ServerOptions().setPort(port));
        setWorker(TP);
        use(new CorsHandler(new Cors().setOrigin(allowedOrigins)));
        error(this::handleError);
        install(new OpenAPIModule());
        install(new JacksonModule());

        mount(new FormController(state));
        mount(new ViewController(state));
        mount(new EventController(state));
        mount(new WsController(state));
    }

    public void handleError(Context ctx, Throwable cause, StatusCode statusCode) {
        String errorMessage;
        ctx.setResponseCode(statusCode);

        if (statusCode.value() == 500) {
            LOG.error("Error: {}", statusCode, cause);
            errorMessage = ERROR_UNKNOWN;
        } else {
            String message = "Error: " + statusCode;
            LOG.error(message);
            errorMessage = cause.getMessage();
        }
        ctx.render(new ServiceView(statusCode.value(), errorMessage));
    }
}
