import chess.ChessBoard;
import com.zaxxer.hikari.HikariDataSource;
import redis.clients.jedis.ConnectionPoolConfig;
import services.broadcast.Broadcaster;
import services.broadcast.LocalBroadcaster;
import services.daos.ChallengeDao;
import services.daos.DictionaryDao;
import services.daos.ReplayDao;
import services.daos.UserDao;
import redis.clients.jedis.JedisPooled;
import services.game.GameService;
import services.producers.ChallengeProducer;
import utils.Config;
import web.controllers.AppController;
import web.State;
import web.reusable.AuthService;
import web.reusable.PathService;

import java.util.List;
import java.util.Map;

import static io.jooby.Jooby.runApp;
import static utils.Globals.JSON_MAPPER;
import static utils.Globals.LOG;

public class Main {
    public static void main(String[] args) throws Exception {
        ConnectionPoolConfig poolConfig = Config.getJedisPoolConfig();

        Map<String, String> env = Config.readEnvironment();
        HikariDataSource ds = Config.createDataSource(env);

        String cookieDomain = env.get("COOKIE_DOMAIN");

        String allowedOriginsStr = env.get("ALLOWED_ORIGINS");
        List<String> allowedOrigins = List.of(allowedOriginsStr.split(","));

        int port = Integer.parseInt(env.get("APP_PORT"));

        String redisHost = env.get("REDIS_HOST");
        int redisPort = Integer.parseInt(env.get("REDIS_PORT"));
        String redisPubsubHost = env.get("REDIS_PUBSUB_HOST");
        int redisPubsubPort = Integer.parseInt(env.get("REDIS_PUBSUB_PORT"));

        List<String> countryList = Config.createCountryList();

        State state = new State();

        UserDao userDao = new UserDao(ds);
        ReplayDao replayDao = new ReplayDao(ds);
        ChallengeDao challengeDao = new ChallengeDao(ds);
        DictionaryDao dictionaryDao = new DictionaryDao(new JedisPooled(poolConfig, redisHost, redisPort));
        GameService gameService = new GameService(dictionaryDao, userDao, replayDao);
        AuthService authService = new AuthService(dictionaryDao, cookieDomain);
//        GlobalBroadcaster gameBroadcaster = new GlobalBroadcaster(poolConfig, redisPubsubHost, redisPubsubPort, Broadcaster.GAMES_TOPIC);
//        GlobalBroadcaster userBroadcaster = new GlobalBroadcaster(poolConfig, redisPubsubHost, redisPubsubPort, Broadcaster.USERS_TOPIC);
        LocalBroadcaster gameBroadcaster = new LocalBroadcaster(Broadcaster.GAMES_TOPIC);
        LocalBroadcaster userBroadcaster = new LocalBroadcaster(Broadcaster.USERS_TOPIC);
        ChallengeProducer challengeProducer = new ChallengeProducer(userBroadcaster);
        PathService pathService = new PathService();

        state.setUserDao(userDao);
        state.setReplayDao(replayDao);
        state.setChallengeDao(challengeDao);
        state.setDictionaryDao(dictionaryDao);
        state.setGameService(gameService);
        state.setAuthService(authService);
        state.setGameBroadcaster(gameBroadcaster);
        state.setUserBroadcaster(userBroadcaster);
        state.setChallengeProducer(challengeProducer);
        state.setCountryList(countryList);
        state.setPathService(pathService);
        state.setInitialBoard(ChessBoard.initial());

//        gameBroadcaster.startListenSubscribe();
//        userBroadcaster.startListenSubscribe();

        LOG.info("Starting on port {} with allowedOrigins={}", port, allowedOrigins);
        runApp(args, () -> new AppController(port, allowedOrigins, state));
    }
}
