package scripts;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.core.type.TypeReference;
import com.zaxxer.hikari.HikariDataSource;
import chess.PieceMove;
import lombok.SneakyThrows;
import services.daos.DictionaryDao;
import services.daos.ChallengeDao;
import services.daos.ReplayDao;
import services.daos.UserDao;
import lombok.AllArgsConstructor;
import org.apache.commons.dbutils.QueryRunner;
import redis.clients.jedis.JedisPooled;
import utils.Config;

import java.net.URI;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.TimeUnit;
import java.util.function.Consumer;

import static utils.Globals.*;
import static services.daos.UserDao.*;
import static services.daos.ReplayDao.*;
import static services.daos.ChallengeDao.*;

@AllArgsConstructor
public class DatabaseSeed {

    private static final String USERS_JSON = readResourceAsString("/seed/users.json");
    private static final String REPLAYS_JSON = readResourceAsString("/seed/replays.json");
    private static final String CHALLENGES_JSON = readResourceAsString("/seed/challenges.json");

    private static final TypeReference<List<UserInst>> USER_LIST_TYPE = new TypeReference<>() {};
    private static final TypeReference<List<ReplayInst>> REPLAY_LIST_TYPE = new TypeReference<>() {};
    private static final TypeReference<List<ChallengeInst>> CHALLENGE_LIST_TYPE = new TypeReference<>() {};

    @SneakyThrows
    public static String readResourceAsString(String resourcePath)  {
        URI uri = Objects.requireNonNull(DatabaseSeed.class.getResource(resourcePath)).toURI();
        Path filePath = Path.of(uri);
        return Files.readString(filePath);
    }

    private static String randomGameStateAsJson() {
        try {
            List<PieceMove> moveList = PieceMove.randomMoveList();
            return JSON.writeValueAsString(moveList);
        } catch (JsonProcessingException ex) {
            throw new RuntimeException(ex);
        }
    }

    private static <T> void seedTableInParallel(List<T> insts, Consumer<T> consumer) {
        var futures = insts.stream()
            .map((inst) -> CompletableFuture.runAsync(() -> consumer.accept(inst), EXECUTOR))
            .toList();
        futures.forEach((f) -> {
            try {
                f.get(MAX_WAIT_MS, TimeUnit.MILLISECONDS);
            } catch (Exception e) {
                throw new RuntimeException(e);
            }
        });
    }

    public static void main(String[] args) throws Exception {
        long startTime = System.currentTimeMillis();

        Map<String, String> env = Config.readEnvironment();
        HikariDataSource ds = Config.createDataSource(env);
        new QueryRunner(ds).execute("BEGIN; DROP SCHEMA public CASCADE; CREATE SCHEMA public; END;");

        Config.createSchema(ds);

        String redisHost = env.get("REDIS_HOST");
        int redisPort = Integer.parseInt(env.get("REDIS_PORT"));
        JedisPooled jedis = new JedisPooled(redisHost, redisPort);

        jedis.flushAll();

        UserDao userDao = new UserDao(ds);
        ChallengeDao challengeDao = new ChallengeDao(ds);
        ReplayDao replayDao = new ReplayDao(ds);
        DictionaryDao dictionaryDao = new DictionaryDao(jedis);

        List<UserInst> userInsts = JSON.readValue(USERS_JSON, USER_LIST_TYPE);
        List<ReplayInst> replayInsts = JSON.readValue(REPLAYS_JSON, REPLAY_LIST_TYPE)
            .stream()
            .map(replay -> replay.withMoveListJson(randomGameStateAsJson()))
            .toList();
        List<ChallengeInst> challengeInsts = JSON.readValue(CHALLENGES_JSON, CHALLENGE_LIST_TYPE);

        userDao.batchInsert(userInsts);
        seedTableInParallel(replayInsts, replayDao::insert);
        seedTableInParallel(challengeInsts, challengeDao::insert);

        DictionaryDao.EloChangeSet[] changeSets = userDao.getAll()
            .stream()
            .map(user -> new DictionaryDao.EloChangeSet(user.getId(), user.getElo()))
            .toArray(DictionaryDao.EloChangeSet[]::new);
        dictionaryDao.incrLeaderboardUser(changeSets);

        long endTime = System.currentTimeMillis() - startTime;
        LOG.info("Took {} ms to execute seeding script", endTime);

        jedis.close();
        ds.close();

        System.exit(0); // force shutdown
    }
}
