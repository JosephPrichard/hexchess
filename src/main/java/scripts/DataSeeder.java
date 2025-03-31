package scripts;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.zaxxer.hikari.HikariDataSource;
import dao.ChallengeDao;
import dao.HistoryDao;
import dao.UserDao;
import domain.Move;
import lombok.AllArgsConstructor;
import models.GameState;
import models.User;
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

@AllArgsConstructor
public class DataSeeder {

    private final DataSource ds;
    private final RemoteDict remoteDict;

    private static final List<UserDao.UserInst> USER_INSTS = List.of(
            new UserDao.UserInst("id1", "User1", "password1", "us", 1000f, 0, 0),
            new UserDao.UserInst("id2", "User2", "password2", "us", 1005f, 1, 0),
            new UserDao.UserInst("id3", "User3", "password3", "us", 900f, 1, 8),
            new UserDao.UserInst("id4", "User4", "password4", "us", 2000f, 50, 20),
            new UserDao.UserInst("id5", "User5", "password5", "us", 1500f, 40, 35),
            new UserDao.UserInst("id6", "JohnDoe", "password6", "fr", 1700f, 60, 10),
            new UserDao.UserInst("id7", "JaneSmith", "password7", "dk", 1100f, 5, 2),
            new UserDao.UserInst("id8", "AliceWonder", "password8", "au", 1200f, 3, 5),
            new UserDao.UserInst("id9", "BobBuilder", "password9", "us", 1800f, 30, 25),
            new UserDao.UserInst("id10", "CharlieBrown", "password10", "fr", 1900f, 15, 12),
            new UserDao.UserInst("id11", "DavidKing", "password11", "dk", 1050f, 2, 1),
            new UserDao.UserInst("id12", "EmmaGreen", "password12", "au", 950f, 20, 18),
            new UserDao.UserInst("id13", "FrankWhite", "password13", "us", 2100f, 12, 5),
            new UserDao.UserInst("id14", "GeorgeClark", "password14", "fr", 1650f, 9, 4),
            new UserDao.UserInst("id15", "HarryPotter", "password15", "dk", 2500f, 55, 30),
            new UserDao.UserInst("id16", "IslaFisher", "password16", "au", 1450f, 7, 6),
            new UserDao.UserInst("id17", "JackSparrow", "password17", "us", 1300f, 1, 0),
            new UserDao.UserInst("id18", "KateBishop", "password18", "fr", 1150f, 10, 8),
            new UserDao.UserInst("id19", "LukeSkywalker", "password19", "dk", 980f, 4, 2),
            new UserDao.UserInst("id20", "MariaShar", "password20", "au", 1325f, 22, 15),
            new UserDao.UserInst("id21", "NancyDrew", "password21", "us", 1880f, 35, 25),
            new UserDao.UserInst("id22", "OliverTwine", "password22", "fr", 1720f, 18, 14),
            new UserDao.UserInst("id23", "PeterPan", "password23", "dk", 2025f, 40, 28),
            new UserDao.UserInst("id24", "QuinnBryant", "password24", "au", 1455f, 5, 4),
            new UserDao.UserInst("id25", "RachelGreen", "password25", "us", 2100f, 65, 55),
            new UserDao.UserInst("id26", "SamuelJackson", "password26", "fr", 1375f, 12, 9),
            new UserDao.UserInst("id27", "TonyStark", "password27", "dk", 1650f, 25, 18),
            new UserDao.UserInst("id28", "UrsulaStein", "password28", "au", 980f, 8, 7),
            new UserDao.UserInst("id29", "VictorHugo", "password29", "us", 1220f, 20, 15),
            new UserDao.UserInst("id30", "WalterWhite", "password30", "fr", 1685f, 45, 40),
            new UserDao.UserInst("id31", "XanderHarris", "password31", "dk", 1125f, 3, 1),
            new UserDao.UserInst("id32", "YaraGrey", "password32", "au", 1590f, 17, 12),
            new UserDao.UserInst("id33", "ZeusKing", "password33", "us", 1845f, 30, 22),
            new UserDao.UserInst("id34", "AmeliaClark", "password34", "fr", 1410f, 9, 7),
            new UserDao.UserInst("id35", "BradPitt", "password35", "dk", 2200f, 70, 50),
            new UserDao.UserInst("id36", "ClaireRedfield", "password36", "au", 1800f, 32, 28),
            new UserDao.UserInst("id37", "DonDrake", "password37", "us", 1950f, 18, 12),
            new UserDao.UserInst("id38", "EvaGreen", "password38", "fr", 2000f, 27, 21),
            new UserDao.UserInst("id39", "FionaApple", "password39", "dk", 1700f, 14, 9),
            new UserDao.UserInst("id40", "GregoryHouse", "password40", "au", 1540f, 20, 15),
            new UserDao.UserInst("id41", "HankSch", "password41", "us", 1880f, 33, 24),
            new UserDao.UserInst("id42", "IreneAdler", "password42", "fr", 1300f, 11, 6),
            new UserDao.UserInst("id43", "JohnSnow", "password43", "dk", 1990f, 36, 28),
            new UserDao.UserInst("id44", "KarenSmith", "password44", "au", 1500f, 12, 8),
            new UserDao.UserInst("id45", "LeoMessi", "password45", "us", 1750f, 25, 18),
            new UserDao.UserInst("id46", "MikeTyson", "password46", "fr", 1420f, 8, 5),
            new UserDao.UserInst("id47", "NancySmith", "password47", "dk", 1600f, 6, 4),
            new UserDao.UserInst("id48", "OscarWilde", "password48", "au", 1900f, 20, 10),
            new UserDao.UserInst("id49", "PaulaDean", "password49", "us", 1755f, 18, 12),
            new UserDao.UserInst("id50", "QuincyJones", "password50", "fr", 1400f, 5, 2),
            new UserDao.UserInst("id51", "RalphLauren", "password51", "dk", 2000f, 28, 18),
            new UserDao.UserInst("id52", "SallyField", "password52", "au", 1550f, 14, 9),
            new UserDao.UserInst("id53", "ThomasEdison", "password53", "us", 1875f, 22, 15),
            new UserDao.UserInst("id54", "UmaThurman", "password54", "fr", 1350f, 8, 5),
            new UserDao.UserInst("id55", "VictorFranken", "password55", "dk", 1655f, 16, 12),
            new UserDao.UserInst("id56", "WandaMaximoff", "password56", "au", 1250f, 10, 7),
            new UserDao.UserInst("id57", "XenaWarrior", "password57", "us", 1495f, 12, 8),
            new UserDao.UserInst("id58", "YasminePerez", "password58", "fr", 1450f, 7, 3),
            new UserDao.UserInst("id59", "ZackFair", "password59", "dk", 1750f, 20, 15),
            new UserDao.UserInst("id60", "AlbertEinstein", "password60", "au", 1980f, 35, 25),
            new UserDao.UserInst("id61", "BruceBanner", "password61", "us", 2100f, 60, 45),
            new UserDao.UserInst("id62", "CatherineZeta", "password62", "fr", 1300f, 9, 6),
            new UserDao.UserInst("id63", "DianaPrince", "password63", "dk", 2500f, 70, 50),
            new UserDao.UserInst("id64", "EdwardScissor", "password64", "au", 1555f, 15, 12),
            new UserDao.UserInst("id65", "FrodoBaggins", "password65", "us", 1600f, 18, 14),
            new UserDao.UserInst("id66", "GandalfWhite", "password66", "fr", 1450f, 12, 8),
            new UserDao.UserInst("id67", "HermioneGranger", "password67", "dk", 1950f, 25, 20),
            new UserDao.UserInst("id68", "IndianaJones", "password68", "au", 1300f, 10, 6),
            new UserDao.UserInst("id69", "JamesBond", "password69", "us", 1800f, 30, 20),
            new UserDao.UserInst("id70", "KatnissEverdeen", "password70", "fr", 1520f, 18, 12),
            new UserDao.UserInst("id71", "LoganWolverine", "password71", "dk", 1400f, 12, 10),
            new UserDao.UserInst("id72", "MorpheusMatrix", "password72", "au", 1620f, 16, 12),
            new UserDao.UserInst("id73", "NeoAnderson", "password73", "us", 2000f, 28, 18),
            new UserDao.UserInst("id74", "OptimusPrime", "password74", "fr", 1550f, 14, 9),
            new UserDao.UserInst("id75", "PeterParker", "password75", "dk", 1850f, 24, 16),
            new UserDao.UserInst("id76", "QuorraGrid", "password76", "au", 1400f, 10, 8),
            new UserDao.UserInst("id77", "RaphaelNinja", "password77", "us", 1650f, 18, 12),
            new UserDao.UserInst("id78", "SamusAran", "password78", "fr", 1350f, 8, 6),
            new UserDao.UserInst("id79", "TrinityMatrix", "password79", "dk", 1485f, 12, 9),
            new UserDao.UserInst("id80", "UltronPrime", "password80", "au", 1700f, 20, 15),
            new UserDao.UserInst("id81", "VenomSymbiote", "password81", "us", 2000f, 28, 20),
            new UserDao.UserInst("id82", "WolverineX", "password82", "fr", 1500f, 16, 12),
            new UserDao.UserInst("id83", "XavierMind", "password83", "dk", 1800f, 24, 18),
            new UserDao.UserInst("id84", "YodaMaster", "password84", "au", 1320f, 14, 10),
            new UserDao.UserInst("id85", "ZorroBlade", "password85", "us", 1750f, 20, 14));

    private static final List<HistoryDao.HistoryInst> HISTORY_INSTS = List.of(
        new HistoryDao.HistoryInst("id37", "id5", 1, 15d, -15d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id2", "id27", 0, 10d, -10d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id4", "id19", 2, 0d, 0d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id12", "id32", 1, 20d, -20d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id29", "id8", 0, 30d, -30d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id14", "id23", 2, 0d, 0d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id30", "id10", 1, 18d, -18d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id7", "id33", 0, 23d, -23d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id21", "id25", 2, 0d, 0d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id9", "id11", 1, 16d, -16d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id1", "id18", 0, 28d, -28d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id24", "id6", 2, 0d, 0d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id15", "id20", 1, 30d, -30d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id35", "id31", 0, 25d, -25d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id26", "id3", 2, 0d, 0d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id28", "id17", 1, 11d, -11d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id13", "id16", 0, 22d, -22d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id3", "id34", 2, 0d, 0d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id11", "id22", 1, 13d, -13d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id34", "id38", 0, 27d, -27d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id36", "id1", 2, 0d, 0d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id31", "id7", 1, 19d, -19d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id39", "id10", 0, 22d, -22d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id22", "id4", 2, 0d, 0d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id20", "id15", 1, 27d, -27d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id8", "id14", 0, 18d, -18d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id18", "id9", 2, 0d, 0d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id5", "id13", 1, 24d, -24d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id16", "id30", 0, 25d, -25d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id33", "id19", 2, 0d, 0d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id24", "id7", 1, 28d, -28d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id6", "id21", 0, 22d, -22d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id8", "id2", 1, 19d, -19d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id1", "id2", 1, 15d, -15d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id2", "id1", 0, 10d, -10d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id1", "id2", 2, 0d, 0d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id2", "id1", 1, 20d, -20d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id1", "id2", 0, 30d, -30d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id2", "id1", 2, 0d, 0d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id1", "id2", 1, 18d, -18d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id2", "id1", 0, 23d, -23d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id1", "id2", 2, 0d, 0d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id2", "id1", 1, 16d, -16d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id1", "id2", 0, 28d, -28d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id2", "id1", 2, 0d, 0d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id1", "id2", 1, 30d, -30d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id2", "id1", 0, 25d, -25d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id1", "id2", 2, 0d, 0d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id2", "id1", 1, 11d, -11d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id1", "id2", 0, 22d, -22d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id2", "id1", 2, 0d, 0d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id1", "id2", 1, 13d, -13d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id2", "id1", 0, 27d, -27d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id1", "id2", 1, 24d, -24d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id2", "id1", 0, 19d, -19d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id1", "id2", 2, 0d, 0d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id2", "id1", 1, 26d, -26d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id1", "id2", 0, 21d, -21d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id2", "id1", 2, 0d, 0d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id1", "id2", 1, 28d, -28d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id2", "id1", 0, 15d, -15d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id1", "id2", 2, 0d, 0d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id2", "id1", 1, 12d, -12d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id1", "id2", 0, 18d, -18d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id2", "id1", 2, 0d, 0d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id1", "id2", 1, 23d, -23d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id2", "id1", 0, 17d, -17d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id1", "id2", 2, 0d, 0d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id2", "id1", 1, 27d, -27d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id1", "id2", 0, 20d, -20d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id2", "id1", 2, 0d, 0d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id1", "id2", 1, 14d, -14d, randomGameStateAsJson()),
        new HistoryDao.HistoryInst("id2", "id1", 0, 30d, -30d, randomGameStateAsJson()));

    private static final List<Pair<String, String>> CHALLENGE_INSTS = List.of(
        Pair.of("id1", "id2"),
        Pair.of("id1", "id3"),
        Pair.of("id1", "id4"),
        Pair.of("id1", "id5"),
        Pair.of("id1", "id6"),
        Pair.of("id1", "id7"),
        Pair.of("id2", "id1"),
        Pair.of("id3", "id1"),
        Pair.of("id4", "id1"));

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

    private void seedUsersTable(List<UserDao.UserInst> insts) {
        UserDao userDao = new UserDao(ds);
        seedTable(insts, userDao::insert);
    }

    private void seedHistTable(List<HistoryDao.HistoryInst> insts) {
        HistoryDao histDao = new HistoryDao(ds);
        seedTable(insts, histDao::insert);
    }

    private void seedChallengeTable(List<Pair<String, String>> insts) {
        ChallengeDao challengeDao = new ChallengeDao(ds);
        seedTable(insts, (pair) -> challengeDao.insert(pair.getLeft(), pair.getRight()));
    }

    private void seedUsersDict() {
        UserDao userDao = new UserDao(ds);
        List<User> allUsers = userDao.getAll();

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

        DataSeeder seeder = new DataSeeder(ds, remoteDict);
        seeder.seedUsersTable(USER_INSTS);
        seeder.seedUsersDict();
        seeder.seedHistTable(HISTORY_INSTS);
        seeder.seedChallengeTable(CHALLENGE_INSTS);

        long endTime = System.currentTimeMillis() - startTime;
        LOGGER.info("Took {} ms to execute seeding script", endTime);

//        jedis.close();
        ds.close();
    }
}
