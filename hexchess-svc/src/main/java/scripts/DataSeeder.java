package scripts;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.zaxxer.hikari.HikariDataSource;
import chess.PieceMove;
import services.daos.DictionaryDao;
import services.daos.ChallengeDao;
import services.daos.ReplayDao;
import services.daos.UserDao;
import lombok.AllArgsConstructor;
import models.entities.UserEntity;
import org.apache.commons.dbutils.QueryRunner;
import redis.clients.jedis.JedisPooled;
import utils.Config;

import javax.sql.DataSource;
import java.io.IOException;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
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
public class DataSeeder {

    private final DataSource ds;
    private final DictionaryDao dictionaryDao;

    public static String readResourceAsString(String resourcePath) throws IOException {
        try (InputStream inputStream = DataSeeder.class.getResourceAsStream(resourcePath)) {
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
            return JSON_MAPPER.writeValueAsString(moveList);
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
        List<UserInst> insts = JSON_MAPPER.readValue(json,
            JSON_MAPPER.getTypeFactory().constructCollectionType(ArrayList.class, UserInst.class));
        UserDao userDao = new UserDao(ds);
        insts.forEach(userDao::insert);
    }

    private void seedReplayTable() throws IOException {
        String json = readResourceAsString("/seed/replays.json");
        List<ReplayInst> insts = JSON_MAPPER.readValue(json,
            JSON_MAPPER.getTypeFactory().constructCollectionType(ArrayList.class, ReplayInst.class));
        insts.forEach(replay -> replay.moveListJson = randomGameStateAsJson());
        ReplayDao replayDao = new ReplayDao(ds);
        seedTableInParallel(insts, replayDao::insert);
    }

    private void seedChallengeTable() throws IOException {
        String json = readResourceAsString("/seed/challenges.json");
        List<ChallengeInst> insts = JSON_MAPPER.readValue(json,
            JSON_MAPPER.getTypeFactory().constructCollectionType(ArrayList.class, ChallengeInst.class));
        ChallengeDao challengeDao = new ChallengeDao(ds);
        seedTableInParallel(insts, challengeDao::insert);
    }

    private void seedUsersDict() {
        UserDao userDao = new UserDao(ds);
        List<UserEntity> allUsers = userDao.getAll();

        DictionaryDao.EloChangeSet[] changeSets = allUsers.stream()
            .map(user -> new DictionaryDao.EloChangeSet(user.id, user.elo))
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

        DataSeeder seeder = new DataSeeder(ds, dictionaryDao);
        seeder.seedUsersTable();
        seeder.seedUsersDict();
        seeder.seedReplayTable();
        seeder.seedChallengeTable();

        long endTime = System.currentTimeMillis() - startTime;
        LOGGER.info("Took {} ms to execute seeding script", endTime);

//        jedis.close();
        ds.close();
    }
}
