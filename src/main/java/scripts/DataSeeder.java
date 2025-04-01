package scripts;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.zaxxer.hikari.HikariDataSource;
import dao.ChallengeDao;
import dao.ReplayDao;
import dao.UserDao;
import domain.Move;
import lombok.AllArgsConstructor;
import models.GameState;
import models.UserEntity;
import org.apache.commons.dbutils.QueryRunner;
import org.apache.commons.lang3.tuple.Pair;
import redis.clients.jedis.JedisPooled;
import services.RemoteDict;
import utils.Config;

import javax.sql.DataSource;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ExecutionException;
import java.util.function.Consumer;

import static utils.Globals.*;
import static dao.UserDao.*;
import static dao.ReplayDao.*;

@AllArgsConstructor
public class DataSeeder {

    private final DataSource ds;
    private final RemoteDict remoteDict;

    private static final List<UserInst> USER_INSTS = List.of(
        new UserInst("User1", "password1", "us", 1000f, 0, 0),
        new UserInst("User2", "password2", "us", 1005f, 1, 0),
        new UserInst("User3", "password3", "us", 900f, 1, 8),
        new UserInst("User4", "password4", "us", 2000f, 50, 20),
        new UserInst("User5", "password5", "us", 1500f, 40, 35),
        new UserInst("JohnDoe", "password6", "fr", 1700f, 60, 10),
        new UserInst("JaneSmith", "password7", "dk", 1100f, 5, 2),
        new UserInst("AliceWonder", "password8", "au", 1200f, 3, 5),
        new UserInst("BobBuilder", "password9", "us", 1800f, 30, 25),
        new UserInst("CharlieBrown", "password10", "fr", 1900f, 15, 12),
        new UserInst("DavidKing", "password11", "dk", 1050f, 2, 1),
        new UserInst("EmmaGreen", "password12", "au", 950f, 20, 18),
        new UserInst("FrankWhite", "password13", "us", 2100f, 12, 5),
        new UserInst("GeorgeClark", "password14", "fr", 1650f, 9, 4),
        new UserInst("HarryPotter", "password15", "dk", 2500f, 55, 30),
        new UserInst("IslaFisher", "password16", "au", 1450f, 7, 6),
        new UserInst("JackSparrow", "password17", "us", 1300f, 1, 0),
        new UserInst("KateBishop", "password18", "fr", 1150f, 10, 8),
        new UserInst("LukeSkywalker", "password19", "dk", 980f, 4, 2),
        new UserInst("MariaShar", "password20", "au", 1325f, 22, 15),
        new UserInst("NancyDrew", "password21", "us", 1880f, 35, 25),
        new UserInst("OliverTwine", "password22", "fr", 1720f, 18, 14),
        new UserInst("PeterPan", "password23", "dk", 2025f, 40, 28),
        new UserInst("QuinnBryant", "password24", "au", 1455f, 5, 4),
        new UserInst("RachelGreen", "password25", "us", 2100f, 65, 55),
        new UserInst("SamuelJackson", "password26", "fr", 1375f, 12, 9),
        new UserInst("TonyStark", "password27", "dk", 1650f, 25, 18),
        new UserInst("UrsulaStein", "password28", "au", 980f, 8, 7),
        new UserInst("VictorHugo", "password29", "us", 1220f, 20, 15),
        new UserInst("WalterWhite", "password30", "fr", 1685f, 45, 40),
        new UserInst("XanderHarris", "password31", "dk", 1125f, 3, 1),
        new UserInst("YaraGrey", "password32", "au", 1590f, 17, 12),
        new UserInst("ZeusKing", "password33", "us", 1845f, 30, 22),
        new UserInst("AmeliaClark", "password34", "fr", 1410f, 9, 7),
        new UserInst("BradPitt", "password35", "dk", 2200f, 70, 50),
        new UserInst("ClaireRedfield", "password36", "au", 1800f, 32, 28),
        new UserInst("DonDrake", "password37", "us", 1950f, 18, 12),
        new UserInst("EvaGreen", "password38", "fr", 2000f, 27, 21),
        new UserInst("FionaApple", "password39", "dk", 1700f, 14, 9),
        new UserInst("GregoryHouse", "password40", "au", 1540f, 20, 15),
        new UserInst("HankSch", "password41", "us", 1880f, 33, 24),
        new UserInst("IreneAdler", "password42", "fr", 1300f, 11, 6),
        new UserInst("JohnSnow", "password43", "dk", 1990f, 36, 28),
        new UserInst("KarenSmith", "password44", "au", 1500f, 12, 8),
        new UserInst("LeoMessi", "password45", "us", 1750f, 25, 18),
        new UserInst("MikeTyson", "password46", "fr", 1420f, 8, 5),
        new UserInst("NancySmith", "password47", "dk", 1600f, 6, 4),
        new UserInst("OscarWilde", "password48", "au", 1900f, 20, 10),
        new UserInst("PaulaDean", "password49", "us", 1755f, 18, 12),
        new UserInst("QuincyJones", "password50", "fr", 1400f, 5, 2),
        new UserInst("RalphLauren", "password51", "dk", 2000f, 28, 18),
        new UserInst("SallyField", "password52", "au", 1550f, 14, 9),
        new UserInst("ThomasEdison", "password53", "us", 1875f, 22, 15),
        new UserInst("UmaThurman", "password54", "fr", 1350f, 8, 5),
        new UserInst("VictorFranken", "password55", "dk", 1655f, 16, 12),
        new UserInst("WandaMaximoff", "password56", "au", 1250f, 10, 7),
        new UserInst("XenaWarrior", "password57", "us", 1495f, 12, 8),
        new UserInst("YasminePerez", "password58", "fr", 1450f, 7, 3),
        new UserInst("ZackFair", "password59", "dk", 1750f, 20, 15),
        new UserInst("AlbertEinstein", "password60", "au", 1980f, 35, 25),
        new UserInst("BruceBanner", "password61", "us", 2100f, 60, 45),
        new UserInst("CatherineZeta", "password62", "fr", 1300f, 9, 6),
        new UserInst("DianaPrince", "password63", "dk", 2500f, 70, 50),
        new UserInst("EdwardScissor", "password64", "au", 1555f, 15, 12),
        new UserInst("FrodoBaggins", "password65", "us", 1600f, 18, 14),
        new UserInst("GandalfWhite", "password66", "fr", 1450f, 12, 8),
        new UserInst("HermioneGranger", "password67", "dk", 1950f, 25, 20),
        new UserInst("IndianaJones", "password68", "au", 1300f, 10, 6),
        new UserInst("JamesBond", "password69", "us", 1800f, 30, 20),
        new UserInst("KatnissEverdeen", "password70", "fr", 1520f, 18, 12),
        new UserInst("LoganWolverine", "password71", "dk", 1400f, 12, 10),
        new UserInst("MorpheusMatrix", "password72", "au", 1620f, 16, 12),
        new UserInst("NeoAnderson", "password73", "us", 2000f, 28, 18),
        new UserInst("OptimusPrime", "password74", "fr", 1550f, 14, 9),
        new UserInst("PeterParker", "password75", "dk", 1850f, 24, 16),
        new UserInst("QuorraGrid", "password76", "au", 1400f, 10, 8),
        new UserInst("RaphaelNinja", "password77", "us", 1650f, 18, 12),
        new UserInst("SamusAran", "password78", "fr", 1350f, 8, 6),
        new UserInst("TrinityMatrix", "password79", "dk", 1485f, 12, 9),
        new UserInst("UltronPrime", "password80", "au", 1700f, 20, 15),
        new UserInst("VenomSymbiote", "password81", "us", 2000f, 28, 20),
        new UserInst("WolverineX", "password82", "fr", 1500f, 16, 12),
        new UserInst("XavierMind", "password83", "dk", 1800f, 24, 18),
        new UserInst("YodaMaster", "password84", "au", 1320f, 14, 10),
        new UserInst("ZorroBlade", "password85", "us", 1750f, 20, 14));

    private static final List<ReplayInst> REPLAY_INSTS = List.of(
        new ReplayInst(37L, 5L, 1, 15d, -15d, randomGameStateAsJson()),
        new ReplayInst(2L, 27L, 0, 10d, -10d, randomGameStateAsJson()),
        new ReplayInst(4L, 19L, 2, 0d, 0d, randomGameStateAsJson()),
        new ReplayInst(12L, 32L, 1, 20d, -20d, randomGameStateAsJson()),
        new ReplayInst(29L, 8L, 0, 30d, -30d, randomGameStateAsJson()),
        new ReplayInst(14L, 23L, 2, 0d, 0d, randomGameStateAsJson()),
        new ReplayInst(30L, 10L, 1, 18d, -18d, randomGameStateAsJson()),
        new ReplayInst(7L, 33L, 0, 23d, -23d, randomGameStateAsJson()),
        new ReplayInst(21L, 25L, 2, 0d, 0d, randomGameStateAsJson()),
        new ReplayInst(9L, 11L, 1, 16d, -16d, randomGameStateAsJson()),
        new ReplayInst(1L, 18L, 0, 28d, -28d, randomGameStateAsJson()),
        new ReplayInst(24L, 6L, 2, 0d, 0d, randomGameStateAsJson()),
        new ReplayInst(15L, 20L, 1, 30d, -30d, randomGameStateAsJson()),
        new ReplayInst(35L, 31L, 0, 25d, -25d, randomGameStateAsJson()),
        new ReplayInst(26L, 3L, 2, 0d, 0d, randomGameStateAsJson()),
        new ReplayInst(28L, 17L, 1, 11d, -11d, randomGameStateAsJson()),
        new ReplayInst(13L, 16L, 0, 22d, -22d, randomGameStateAsJson()),
        new ReplayInst(3L, 34L, 2, 0d, 0d, randomGameStateAsJson()),
        new ReplayInst(11L, 22L, 1, 13d, -13d, randomGameStateAsJson()),
        new ReplayInst(34L, 38L, 0, 27d, -27d, randomGameStateAsJson()),
        new ReplayInst(36L, 1L, 2, 0d, 0d, randomGameStateAsJson()),
        new ReplayInst(31L, 7L, 1, 19d, -19d, randomGameStateAsJson()),
        new ReplayInst(39L, 10L, 0, 22d, -22d, randomGameStateAsJson()),
        new ReplayInst(22L, 4L, 2, 0d, 0d, randomGameStateAsJson()),
        new ReplayInst(20L, 15L, 1, 27d, -27d, randomGameStateAsJson()),
        new ReplayInst(8L, 14L, 0, 18d, -18d, randomGameStateAsJson()),
        new ReplayInst(18L, 9L, 2, 0d, 0d, randomGameStateAsJson()),
        new ReplayInst(5L, 13L, 1, 24d, -24d, randomGameStateAsJson()),
        new ReplayInst(16L, 30L, 0, 25d, -25d, randomGameStateAsJson()),
        new ReplayInst(33L, 19L, 2, 0d, 0d, randomGameStateAsJson()),
        new ReplayInst(24L, 7L, 1, 28d, -28d, randomGameStateAsJson()),
        new ReplayInst(6L, 21L, 0, 22d, -22d, randomGameStateAsJson()),
        new ReplayInst(8L, 2L, 1, 19d, -19d, randomGameStateAsJson()),
        new ReplayInst(1L, 2L, 1, 15d, -15d, randomGameStateAsJson()),
        new ReplayInst(2L, 1L, 0, 10d, -10d, randomGameStateAsJson()),
        new ReplayInst(1L, 2L, 2, 0d, 0d, randomGameStateAsJson()),
        new ReplayInst(2L, 1L, 1, 20d, -20d, randomGameStateAsJson()),
        new ReplayInst(1L, 2L, 0, 30d, -30d, randomGameStateAsJson()),
        new ReplayInst(2L, 1L, 2, 0d, 0d, randomGameStateAsJson()),
        new ReplayInst(1L, 2L, 1, 18d, -18d, randomGameStateAsJson()),
        new ReplayInst(2L, 1L, 0, 23d, -23d, randomGameStateAsJson()),
        new ReplayInst(1L, 2L, 2, 0d, 0d, randomGameStateAsJson()),
        new ReplayInst(2L, 1L, 1, 16d, -16d, randomGameStateAsJson()),
        new ReplayInst(1L, 2L, 0, 28d, -28d, randomGameStateAsJson()),
        new ReplayInst(2L, 1L, 2, 0d, 0d, randomGameStateAsJson()),
        new ReplayInst(1L, 2L, 1, 30d, -30d, randomGameStateAsJson()),
        new ReplayInst(2L, 1L, 0, 25d, -25d, randomGameStateAsJson()),
        new ReplayInst(1L, 2L, 2, 0d, 0d, randomGameStateAsJson()),
        new ReplayInst(2L, 1L, 1, 11d, -11d, randomGameStateAsJson()),
        new ReplayInst(1L, 2L, 0, 22d, -22d, randomGameStateAsJson()),
        new ReplayInst(2L, 1L, 2, 0d, 0d, randomGameStateAsJson()),
        new ReplayInst(1L, 2L, 1, 13d, -13d, randomGameStateAsJson()),
        new ReplayInst(2L, 1L, 0, 27d, -27d, randomGameStateAsJson()),
        new ReplayInst(1L, 2L, 1, 24d, -24d, randomGameStateAsJson()),
        new ReplayInst(2L, 1L, 0, 19d, -19d, randomGameStateAsJson()),
        new ReplayInst(1L, 2L, 2, 0d, 0d, randomGameStateAsJson()),
        new ReplayInst(2L, 1L, 1, 26d, -26d, randomGameStateAsJson()),
        new ReplayInst(1L, 2L, 0, 21d, -21d, randomGameStateAsJson()),
        new ReplayInst(2L, 1L, 2, 0d, 0d, randomGameStateAsJson()),
        new ReplayInst(1L, 2L, 1, 28d, -28d, randomGameStateAsJson()),
        new ReplayInst(2L, 1L, 0, 15d, -15d, randomGameStateAsJson()),
        new ReplayInst(1L, 2L, 2, 0d, 0d, randomGameStateAsJson()),
        new ReplayInst(2L, 1L, 1, 12d, -12d, randomGameStateAsJson()),
        new ReplayInst(1L, 2L, 0, 18d, -18d, randomGameStateAsJson()),
        new ReplayInst(2L, 1L, 2, 0d, 0d, randomGameStateAsJson()),
        new ReplayInst(1L, 2L, 1, 23d, -23d, randomGameStateAsJson()),
        new ReplayInst(2L, 1L, 0, 17d, -17d, randomGameStateAsJson()),
        new ReplayInst(1L, 2L, 2, 0d, 0d, randomGameStateAsJson()),
        new ReplayInst(2L, 1L, 1, 27d, -27d, randomGameStateAsJson()),
        new ReplayInst(1L, 2L, 0, 20d, -20d, randomGameStateAsJson()),
        new ReplayInst(2L, 1L, 2, 0d, 0d, randomGameStateAsJson()),
        new ReplayInst(1L, 2L, 1, 14d, -14d, randomGameStateAsJson()),
        new ReplayInst(2L, 1L, 0, 30d, -30d, randomGameStateAsJson()));

    private static final List<Pair<Long, Long>> CHALLENGE_INSTS = List.of(
        Pair.of(1L, 2L),
        Pair.of(1L, 3L),
        Pair.of(1L, 4L),
        Pair.of(1L, 5L),
        Pair.of(1L, 6L),
        Pair.of(1L, 7L),
        Pair.of(2L, 1L),
        Pair.of(3L, 1L),
        Pair.of(4L, 1L));

    private static String randomGameStateAsJson() {
        try {
            List<Move> moveList = GameState.randomMoveList();
            return JSON_MAPPER.writeValueAsString(moveList);
        } catch (JsonProcessingException ex) {
            throw new RuntimeException(ex);
        }
    }

    private <T> void seedTable(List<T> insts, Consumer<T> consumer) {
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

    private void seedUsersTable(List<UserInst> insts) {
        UserDao userDao = new UserDao(ds);
        seedTable(insts, userDao::insert);
    }

    private void seedHistTable(List<ReplayInst> insts) {
        ReplayDao histDao = new ReplayDao(ds);
        seedTable(insts, histDao::insert);
    }

    private void seedChallengeTable(List<Pair<Long, Long>> insts) {
        ChallengeDao challengeDao = new ChallengeDao(ds);
        seedTable(insts, (pair) -> challengeDao.insert(pair.getLeft(), pair.getRight()));
    }

    private void seedUsersDict() {
        UserDao userDao = new UserDao(ds);
        List<UserEntity> allUsers = userDao.getAll();

        RemoteDict.EloChangeSet[] changeSets = allUsers.stream()
            .map(user -> new RemoteDict.EloChangeSet(user.getId(), user.getElo()))
            .toArray(RemoteDict.EloChangeSet[]::new);
        remoteDict.incrLeaderboardUser(changeSets);
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
        RemoteDict remoteDict = new RemoteDict(jedis);

        jedis.flushAll();

        DataSeeder seeder = new DataSeeder(ds, remoteDict);
        seeder.seedUsersTable(USER_INSTS);
        seeder.seedUsersDict();
        seeder.seedHistTable(REPLAY_INSTS);
        seeder.seedChallengeTable(CHALLENGE_INSTS);

        long endTime = System.currentTimeMillis() - startTime;
        LOGGER.info("Took {} ms to execute seeding script", endTime);

//        jedis.close();
        ds.close();
    }
}
