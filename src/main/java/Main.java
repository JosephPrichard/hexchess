import com.github.jknack.handlebars.Handlebars;
import com.github.jknack.handlebars.io.ClassPathTemplateLoader;
import com.zaxxer.hikari.HikariDataSource;
import daos.ChallengeDao;
import daos.ReplayDao;
import daos.UserDao;
import redis.clients.jedis.JedisPooled;
import services.*;
import utils.Config;
import web.controllers.AppController;
import web.State;
import web.Templates;

import java.util.List;
import java.util.Map;

import static io.jooby.Jooby.runApp;
import static utils.Globals.LOGGER;

public class Main {
    public static AppController init() {
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
            GlobalBroadcaster gameBroadcaster = new GlobalBroadcaster(jedis, Broadcaster.GAMES_CHANNEL);
            GlobalBroadcaster userBroadcaster = new GlobalBroadcaster(jedis, Broadcaster.USERS_CHANNEL);
            Templates templates = new Templates(handlebars);

            state.setUserDao(userDao);
            state.setReplayDao(replayDao);
            state.setChallengeDao(challengeDao);
            state.setRemoteDict(remoteDict);
            state.setGameService(gameService);
            state.setSessionService(sessionService);
            state.setGameBroadcaster(gameBroadcaster);
            state.setUserBroadcaster(userBroadcaster);
            state.setTemplates(templates);
            state.setCountryList(countryList);

            gameBroadcaster.startListenSubscribe();
            userBroadcaster.startListenSubscribe();

            return new AppController(state);
        } catch (Exception ex) {
            LOGGER.error("Error occurred during router init {}", String.valueOf(ex));
            throw new RuntimeException(ex);
        }
    }

    public static void main(String[] args) {
        runApp(args, Main::init);
    }
}
