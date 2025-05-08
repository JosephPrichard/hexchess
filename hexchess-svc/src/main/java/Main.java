import chess.ChessBoard;
import com.zaxxer.hikari.HikariDataSource;
import services.broadcast.Broadcaster;
import services.broadcast.GlobalBroadcaster;
import services.daos.ChallengeDao;
import services.daos.DictionaryDao;
import services.daos.ReplayDao;
import services.daos.UserDao;
import redis.clients.jedis.JedisPooled;
import services.game.GameService;
import utils.Config;
import web.controllers.AppController;
import web.State;
import web.reusable.PathService;
import web.reusable.SessionService;

import java.util.List;
import java.util.Map;

import static io.jooby.Jooby.runApp;
import static utils.Globals.JSON_MAPPER;

public class Main {
    public static void main(String[] args) throws Exception {
        Map<String, String> env = Config.readEnvironment();
        HikariDataSource ds = Config.createDataSource(env);

        int port = Integer.parseInt(env.get("APP_PORT"));

        String redisHost = env.get("REDIS_HOST");
        int redisPort = Integer.parseInt(env.get("REDIS_PORT"));
        String redisPubsubHost = env.get("REDIS_PUBSUB_HOST");
        int redisPubsubPort = Integer.parseInt(env.get("REDIS_PUBSUB_PORT"));

        List<String> countryList = Config.createCountryList();
        String initialBoardJson = JSON_MAPPER.writeValueAsString(ChessBoard.initial());

        State state = new State();

        UserDao userDao = new UserDao(ds);
        ReplayDao replayDao = new ReplayDao(ds);
        ChallengeDao challengeDao = new ChallengeDao(ds);
        DictionaryDao dictionaryDao = new DictionaryDao(new JedisPooled(redisHost, redisPort));
        GameService gameService = new GameService(dictionaryDao, userDao, replayDao);
        SessionService sessionService = new SessionService();
        GlobalBroadcaster gameBroadcaster = new GlobalBroadcaster(redisPubsubHost, redisPubsubPort, Broadcaster.GAMES_TOPIC);
        GlobalBroadcaster userBroadcaster = new GlobalBroadcaster(redisPubsubHost, redisPubsubPort, Broadcaster.USERS_TOPIC);
        PathService pathService = new PathService();

        state.setUserDao(userDao);
        state.setReplayDao(replayDao);
        state.setChallengeDao(challengeDao);
        state.setDictionaryDao(dictionaryDao);
        state.setGameService(gameService);
        state.setSessionService(sessionService);
        state.setGameBroadcaster(gameBroadcaster);
        state.setUserBroadcaster(userBroadcaster);
        state.setCountryList(countryList);
        state.setPathService(pathService);
        state.setInitialBoardJson(initialBoardJson);

        gameBroadcaster.startListenSubscribe();
        userBroadcaster.startListenSubscribe();

        runApp(args, () -> new AppController(port, state));
    }
}
