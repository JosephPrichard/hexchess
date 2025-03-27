package web;

import com.github.jknack.handlebars.Handlebars;
import com.github.jknack.handlebars.io.ClassPathTemplateLoader;
import com.zaxxer.hikari.HikariDataSource;
import io.jooby.Jooby;
import io.jooby.MediaType;
import io.jooby.ServerOptions;
import io.jooby.exception.NotFoundException;
import io.jooby.jackson.JacksonModule;
import redis.clients.jedis.JedisPooled;
import utils.Config;

import java.util.Map;

import static utils.Globals.LOGGER;

public class Router extends Jooby {

    public static Router init() {
        try {
            HikariDataSource ds = Config.createDataSource();

            String redisHost = System.getenv("REDIS_HOST");
            int redisPort = Integer.parseInt(System.getenv("REDIS_PORT"));
            JedisPooled jedis = new JedisPooled(redisHost, redisPort);

            ClassPathTemplateLoader loader = new ClassPathTemplateLoader();
            loader.setPrefix("/templates");
            loader.setSuffix(".hbs");
            Handlebars handlebars = new Handlebars(loader);

            Map<String, byte[]> files = Config.createFilesMap();

            State state = new State(jedis, ds, handlebars, files);
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
                String message = "Error: " + statusCode + ", " + cause.getMessage();
                LOGGER.error(message);
                ctx.send(cause.getMessage());
            }
        });

        install(new JacksonModule());

        assets("/static/*", "static");

        get("/files/flags/{name}", ctx -> {
            Map<String, byte[]> filesMap = state.getFiles();
            ctx.setResponseType("image/png");

            String name = ctx.path("name").toOptional().orElse("");
            byte[] fileBytes = filesMap.get(name);
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
