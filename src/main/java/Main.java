import com.github.jknack.handlebars.Handlebars;
import com.github.jknack.handlebars.io.ClassPathTemplateLoader;
import com.zaxxer.hikari.HikariDataSource;
import services.dao.ChallengeDao;
import services.dao.ReplayDao;
import services.dao.UserDao;
import redis.clients.jedis.JedisPooled;
import services.GameService;
import services.GlobalBroadcaster;
import services.RemoteDict;
import utils.Config;
import web.routers.Router;
import web.SessionService;
import web.State;
import web.Templates;

import java.util.List;
import java.util.Map;

import static io.jooby.Jooby.runApp;
import static utils.Globals.LOGGER;

public class Main {
    public static Router init() {
        try {
            Map<String, String> env = Config.readEnvironment();
            HikariDataSource ds = Config.createDataSource(env);

            String redisHost = env.get("REDIS_HOST");
            int redisPort = Integer.parseInt(env.get("REDIS_PORT"));
            JedisPooled jedis = new JedisPooled(redisHost, redisPort);

            ClassPathTemplateLoader loader = new ClassPathTemplateLoader();
            loader.setPrefix("/templates");
            loader.setSuffix(".hbs");
            Handlebars handlebars = new Handlebars(loader);

            List<String> countryList = Config.createCountryList();

            State state = new State();

            UserDao userDao = new UserDao(ds);
            ReplayDao replayDao = new ReplayDao(ds);
            ChallengeDao challengeDao = new ChallengeDao(ds);
            RemoteDict remoteDict = new RemoteDict(jedis);
            GameService gameService = new GameService(remoteDict, userDao, replayDao);
            SessionService sessionService = new SessionService();
            GlobalBroadcaster broadcaster = new GlobalBroadcaster(jedis);
            Templates templates = new Templates(handlebars);

            state.setUserDao(userDao);
            state.setReplayDao(replayDao);
            state.setChallengeDao(challengeDao);
            state.setRemoteDict(remoteDict);
            state.setGameService(gameService);
            state.setSessionService(sessionService);
            state.setBroadcaster(broadcaster);
            state.setTemplates(templates);
            state.setCountryList(countryList);

            broadcaster.startListenSubscribe();

            return new Router(state);
        } catch (Exception ex) {
            LOGGER.error("Error occurred during router init {}", String.valueOf(ex));
            throw new RuntimeException(ex);
        }
    }

    public static void main(String[] args) {
        runApp(args, Main::init);
    }
}
