package services.daos;

import models.state.ChessRoom;
import models.state.Player;
import models.entities.RankedEntity;
import models.entities.UserEntity;
import models.views.ChessView;
import redis.clients.jedis.AbstractTransaction;
import redis.clients.jedis.JedisPooled;

import java.time.Duration;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import java.util.Random;

import static utils.Globals.*;

public class DictionaryDao {

    private static final Random RANDOM = new Random();
    private final JedisPooled jedis;

    private static final String GAMES_ZSET = "games";
    private static final byte[] GAMES_ZSET_BYTES = GAMES_ZSET.getBytes();
    private static final String LEADERBOARD_ZSET = "leaderboard";
    private static final String USERS_ZSET = "users";

    private static final Duration USER_EXPIRE_FINISHED = Duration.ofMinutes(1);
    private static final Duration GAME_EXPIRE_FINISHED = Duration.ofHours(1);
    public static final Duration TEMP_SESSION_EXPIRE = Duration.ofMinutes(1);

    public DictionaryDao(JedisPooled jedis) {
        this.jedis = jedis;
    }

    public ChessRoom getRoom(String id) {
        expireRooms();

        String fullId = "game:" + id;
        byte[] bytes = jedis.get(fullId.getBytes());
        if (bytes == null) {
            return null;
        }

        return ChessRoom.deserialize(bytes);
    }

    public ChessRoom setRoom(String id, ChessRoom room) {
        room.setTouch(System.currentTimeMillis());

        byte[] bytes = room.serializeAsBytes();

        id = "game:" + id;

        try (AbstractTransaction t = jedis.multi()) {
            t.set(id.getBytes(), bytes);
            t.zadd(GAMES_ZSET, room.getTouch(), id);
            t.exec();
        }
        return room;
    }

    public void expireRooms() {
        expireRooms(System.currentTimeMillis() - GAME_EXPIRE_FINISHED.toMillis());
    }

    public void expireRooms(long unixTimeExpireMillis) {
        List<String> results = jedis.zrangeByScore(GAMES_ZSET, Double.NEGATIVE_INFINITY, unixTimeExpireMillis);
        String[] gameKeys = results.toArray(String[]::new);

        if (gameKeys.length > 0) {
            LOG.info("Expiring games with keys={}", Arrays.toString(gameKeys));

            try (AbstractTransaction t = jedis.multi()) {
                t.del(gameKeys);
                t.zrem(GAMES_ZSET, gameKeys);
                t.exec();
            }
        }
    }

    public List<ChessView> getUserChessViews(long userId) {
       return List.of();
    }

    public List<ChessView> getChessViews(int page, int count) {
        expireRooms();

        if (page < 1) {
            page = 1;
        }
        int min = (page - 1) * count;
        int max = min + count - 1;

        List<byte[]> elements = jedis.zrevrange(GAMES_ZSET_BYTES, min, max);

        byte[][] fullIds = new byte[elements.size()][];
        for (int i = 0; i < elements.size(); i++) {
            fullIds[i] = elements.get(i);
        }

        List<byte[]> bytesList = null;
        if (fullIds.length > 0) {
            bytesList = jedis.mget(fullIds);
        }
        if (bytesList == null) {
            return List.of();
        }

        List<ChessView> viewList = bytesList.stream().map(ChessView::deserialize).toList();

        LOG.info("Retrieved chess views={} for page={}", viewList, page);
        return viewList;
    }

    public Player getSession(String sessionId) {
        String fullId = "session:" + sessionId;
        byte[] bytes = jedis.get(fullId.getBytes());
        if (bytes == null) {
            return null;
        }
        return Player.deserialize(bytes);
    }

    public void setSession(String sessionId, Player player, long expirySeconds) {
        String fullId = "session:" + sessionId;
        byte[] bytes = player.serializeAsBytes();
        jedis.setex(fullId.getBytes(), expirySeconds, bytes);
    }

    public void updateSessionEx(String sessionId, long expirySeconds) {
        String fullId = "session:" + sessionId;
        jedis.expire(fullId, expirySeconds);
    }

    public void deleteSession(String sessionId) {
        String fullId = "session:" + sessionId;
        jedis.del(fullId);
    }

    public Player getSessionOrDefault(String sessionId) {
        Player player;
        if (sessionId != null) {
            player = getSession(sessionId);
        } else {
            String guestName = "Guest " + RANDOM.nextInt(1000);
            long randomLong = Math.abs(RANDOM.nextLong());
            player = new Player(randomLong, guestName, "", 0, true);
        }
        return player;
    }

    public int getLeaderboardRank(long id) {
        String strId = Long.toString(id);
        Long rank = jedis.zrevrank(LEADERBOARD_ZSET, strId);
        if (rank == null) {
            incrLeaderboardUser(id, UserEntity.START_ELO);
            rank = jedis.zrevrank(LEADERBOARD_ZSET, strId);
            assert rank != null;
        }

        LOG.info("Get leaderboard rank={} of for id={}", rank, id);
        return rank.intValue() + 1;
    }

    public record Leaderboard(List<RankedEntity> users, int pageCount) {}

    public Leaderboard getLeaderboard(int startRank, int count) {
        List<String> ids = jedis.zrevrange(LEADERBOARD_ZSET, startRank, startRank - 1 + count);
        long elemCount = jedis.zcount(LEADERBOARD_ZSET, Integer.MIN_VALUE, Integer.MAX_VALUE);

        long pageCount = (elemCount / count) + Math.min(elemCount % count, 1);

        List<RankedEntity> users = new ArrayList<>();
        for (int i = 0; i < ids.size(); i++) {
            long id = Long.parseUnsignedLong(ids.get(i));
            users.add(new RankedEntity(id, startRank + i + 1));
        }

        return new Leaderboard(users, (int) pageCount);
    }

    public Leaderboard getLeaderboardPage(int page, int perPage) {
        page = Math.max(page, 1);
        int offset = (page - 1) * perPage;
        Leaderboard leaderboard = getLeaderboard(offset, perPage);

        LOG.info("Retrieved leaderboard={} of page={}", leaderboard, page);
        return leaderboard;
    }

    public record EloChangeSet(long id, double elo) {}

    public void incrLeaderboardUser(EloChangeSet... changeSets) {
        try (AbstractTransaction t = jedis.multi()) {
            for (EloChangeSet cs : changeSets) {
                t.zincrby(LEADERBOARD_ZSET, cs.elo(), Long.toString(cs.id()));
            }
            t.exec();
        }
        LOG.info("Incremented leaderboard elo with changeSets={}", Arrays.toString(changeSets));
    }

    public void incrLeaderboardUser(long id, double elo) {
        jedis.zincrby(LEADERBOARD_ZSET, elo, Long.toString(id));
        LOG.info("Incremented leaderboard elo for user={} by elo={}", id, elo);
    }

    public void updateLeaderboardUser(EloChangeSet... changeSets) {
        try (AbstractTransaction t = jedis.multi()) {
            for (EloChangeSet cs : changeSets) {
                t.zadd(LEADERBOARD_ZSET, cs.elo(), Long.toString(cs.id()));
            }
            t.exec();
        }

        LOG.info("Set leaderboard elo with changeSets={}", Arrays.toString(changeSets));
    }

    public void expireUsers() {
        expireUsers(System.currentTimeMillis() - USER_EXPIRE_FINISHED.toMillis());
    }

    public void expireUsers(long unixTimeExpireMillis) {
        List<String> results = jedis.zrangeByScore(USERS_ZSET, Double.NEGATIVE_INFINITY, unixTimeExpireMillis);
        String[] userKeys = results.toArray(String[]::new);

        if (userKeys.length > 0) {
            LOG.info("Expiring users with keys={}", Arrays.toString(userKeys));
            jedis.zrem(USERS_ZSET, userKeys);
        }
    }

    public void addUser(String id) {
        jedis.zadd(USERS_ZSET, System.currentTimeMillis(), id);

        LOG.info("Added user={} into users set", id);
    }

    public void removeUser(String id) {
        jedis.zrem(USERS_ZSET, id);

        LOG.info("Removed user={} from users set", id);
    }

    public long getUsersCount() {
        expireUsers();
        long count = jedis.zcard(USERS_ZSET);

        LOG.info("Counted users with result={}", count);
        return count;
    }

    public long getRoomsCount() {
        expireRooms();
        long count = jedis.zcard(GAMES_ZSET);

        LOG.info("Counted games with result={}", count);
        return count;
    }
}
