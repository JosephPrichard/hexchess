package web;

import com.github.jknack.handlebars.Handlebars;
import com.github.jknack.handlebars.io.ClassPathTemplateLoader;
import io.jooby.Jooby;
import io.jooby.exception.NotFoundException;
import io.jooby.jackson.JacksonModule;
import redis.clients.jedis.JedisPooled;
import utils.Config;

import static utils.Globals.LOGGER;

public class Router extends Jooby {

    public static Router init() {
        try {
            var ds = Config.createDataSource();

            var redisHost = System.getenv("REDIS_HOST");
            var redisPort = Integer.parseInt(System.getenv("REDIS_PORT"));
            var jedis = new JedisPooled(redisHost, redisPort);

            var loader = new ClassPathTemplateLoader();
            loader.setPrefix("/templates");
            loader.setSuffix(".hbs");
            var handlebars = new Handlebars(loader);

            var files = Config.createFilesMap();

            var state = new State(jedis, ds, handlebars, files);
//            var state = new State(null, null, null);
            return new Router(state);
        } catch (Exception ex) {
            LOGGER.error("Error occurred during router init {}", String.valueOf(ex));
            throw new RuntimeException(ex);
        }
    }

    public Router(State state) {
        error((ctx, cause, statusCode) -> {
            ctx.setResponseCode(statusCode);
            if (statusCode.value() == 500) {
                // internal server errors contain stack traces, so log them out to server but do not end to client
                LOGGER.error("Error: {} ", statusCode, cause);
                ctx.send("Internal Server Error");
            } else {
                // non 500 errors contain clear messages that can be spit out as strings to both server logs and the client
                var message = "Error: " + statusCode + ", " + cause.getMessage();
                LOGGER.error(message);
                ctx.send(cause.getMessage());
            }
        });

        install(new JacksonModule());

        // adding the files to the router
        var filesMap = state.getFiles();

        assets("/css/index.css", "/css/index.css");

        assets("/scripts/chess-view.js", "/scripts/chess-view.js");

        assets("/scripts/session.js", "/scripts/session.js");

        get("/files/flags/{name}", ctx -> {
            ctx.setResponseType("image/png");

            var name = ctx.path("name").toOptional().orElse("");
            var fileBytes = filesMap.get(name);
            if (fileBytes == null) {
                throw new NotFoundException("The requested file does not exist: " + name);
            }
            return fileBytes;
        });

        mount(new PageRouter(state));
        mount(new PartialsRouter(state));
        mount(new FormRouter(state));
        mount(new WsRouter(state));
    }
}
