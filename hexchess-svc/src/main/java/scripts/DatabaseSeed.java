package scripts;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.core.type.TypeReference;
import com.zaxxer.hikari.HikariDataSource;
import chess.PieceMove;
import services.daos.DictionaryDao;
import services.daos.ChallengeDao;
import services.daos.ReplayDao;
import services.daos.UserDao;
import lombok.AllArgsConstructor;
import org.apache.commons.dbutils.QueryRunner;
import redis.clients.jedis.JedisPooled;
import utils.Config;
import utils.Globals;

import java.io.IOException;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.Map;
import java.util.Scanner;
import java.util.concurrent.ExecutionException;
import java.util.function.Consumer;

import static utils.Globals.*;
import static services.daos.UserDao.*;
import static services.daos.ReplayDao.*;
import static services.daos.ChallengeDao.*;

@AllArgsConstructor
public class DatabaseSeed {

    private static final TypeReference<List<UserInst>> USER_LIST_TYPE = new TypeReference<>() {};
    private static final TypeReference<List<ReplayInst>> REPLAY_LIST_TYPE = new TypeReference<>() {};
    private static final TypeReference<List<ChallengeInst>> CHALLENGE_LIST_TYPE = new TypeReference<>() {};

    private final UserDao userDao;
    private final ChallengeDao challengeDao;
    private final ReplayDao replayDao;
    private final DictionaryDao dictionaryDao;

    public static String readResourceAsString(String resourcePath) throws IOException {
        try (InputStream inputStream = DatabaseSeed.class.getResourceAsStream(resourcePath)) {
            assert inputStream != null;
            try (Scanner scanner = new Scanner(inputStream, StandardCharsets.UTF_8)) {
                scanner.useDelimiter("\\A");
                return scanner.hasNext() ? scanner.next() : "";
            }
        }
    }

    private static String randomGameStateAsJson() {
        try {
            List<PieceMove> moveList = PieceMove.randomMoveList();
            return JSON.writeValueAsString(moveList);
        } catch (JsonProcessingException ex) {
            throw new RuntimeException(ex);
        }
    }

    private <T> void seedTableInParallel(List<T> insts, Consumer<T> consumer) {
        var futures = insts.stream()
            .map((inst) -> EXECUTOR.submit(() -> consumer.accept(inst)))
            .toList();
        futures.forEach((f) -> {
            try {
                f.get();
            } catch (InterruptedException | ExecutionException e) {
                throw new RuntimeException(e);
            }
        });
    }

    private void seedUsersTable() throws IOException {
        String json = readResourceAsString("/seed/users.json");
        List<UserInst> insts = Globals.JSON.readValue(json, USER_LIST_TYPE);
        insts.forEach(userDao::insert);
    }

    private void seedReplayTable() throws IOException {
        String json = readResourceAsString("/seed/replays.json");
        List<ReplayInst> insts = Globals.JSON.readValue(json, REPLAY_LIST_TYPE)
            .stream()
            .map(replay -> replay.withMoveListJson(randomGameStateAsJson()))
            .toList();
        seedTableInParallel(insts, replayDao::insert);
    }

    private void seedChallengeTable() throws IOException {
        String json = readResourceAsString("/seed/challenges.json");
        List<ChallengeInst> insts = Globals.JSON.readValue(json, CHALLENGE_LIST_TYPE);
        seedTableInParallel(insts, challengeDao::insert);
    }

    private void seedUsersDict() {
        DictionaryDao.EloChangeSet[] changeSets = userDao.getAll()
            .stream()
            .map(user -> new DictionaryDao.EloChangeSet(user.getId(), user.getElo()))
            .toArray(DictionaryDao.EloChangeSet[]::new);
        dictionaryDao.incrLeaderboardUser(changeSets);
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
        DictionaryDao dictionaryDao = new DictionaryDao(jedis);

        jedis.flushAll();

        DatabaseSeed seeder = new DatabaseSeed(new UserDao(ds), new ChallengeDao(ds), new ReplayDao(ds), dictionaryDao);
        seeder.seedUsersTable();
        seeder.seedUsersDict();
        seeder.seedReplayTable();
        seeder.seedChallengeTable();

        long endTime = System.currentTimeMillis() - startTime;
        LOG.info("Took {} ms to execute seeding script", endTime);

//        jedis.close();
        ds.close();
    }
}
