package scripts;

import com.zaxxer.hikari.HikariDataSource;
import redis.clients.jedis.JedisPooled;
import services.daos.DictionaryDao;
import services.daos.UserDao;
import utils.Config;

import java.util.List;
import java.util.Map;

import static utils.Globals.LOG;

public class LeaderboardSync {
    public static final int PER_PAGE = 25;

    public static void main(String[] args) {
        long startTime = System.currentTimeMillis();

        Map<String, String> env = Config.readEnvironment();
        HikariDataSource ds = Config.createDataSource(env);

        UserDao userDao = new UserDao(ds);

        String redisHost = env.get("REDIS_HOST");
        int redisPort = Integer.parseInt(env.get("REDIS_PORT"));
        JedisPooled jedis = new JedisPooled(redisHost, redisPort);
        DictionaryDao dictionaryDao = new DictionaryDao(jedis);

        for (int i = 0;; i++) {
            List<UserDao.EloResult> eloList = userDao.getEloList(i * PER_PAGE, PER_PAGE);
            if (eloList.isEmpty()) {
                break;
            }

            List<DictionaryDao.EloChangeSet> changeSets = eloList.stream()
                .map((pair) -> new DictionaryDao.EloChangeSet(pair.getId(), pair.getElo()))
                .toList();
            dictionaryDao.updateLeaderboardUser(changeSets.toArray(DictionaryDao.EloChangeSet[]::new));
        }

        long endTime = System.currentTimeMillis() - startTime;
        LOG.info("Took {} ms to execute leaderboard sync script", endTime);

        ds.close();
    }
}
