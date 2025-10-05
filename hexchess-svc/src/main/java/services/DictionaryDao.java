package services;

import models.state.ChessState;
import models.state.PlayerState;
import models.entities.UserRankEntity;
import models.entities.UserEntity;
import models.views.ChessView;
import redis.clients.jedis.AbstractTransaction;
import redis.clients.jedis.JedisPooled;

import java.time.Duration;
import java.util.*;

import static utils.Globals.*;

public class DictionaryDao {

    private static final Random RANDOM = new Random();
    private final JedisPooled jedis;

    public static final String GAMES_ZSET = "games";
    public static final String LEADERBOARD_ZSET = "leaderboard";
    public static final String ACTIVE_USERS_ZSET = "active_users";

    private static final Duration USER_EXPIRE_FINISHED = Duration.ofMinutes(2);
    private static final Duration GAME_EXPIRE_FINISHED = Duration.ofHours(1);
    public static final Duration TEMP_SESSION_EXPIRE = Duration.ofMinutes(1);

    public DictionaryDao(JedisPooled jedis) {
        this.jedis = jedis;
    }

    public static String getUserGameZSet(long id) {
        return GAMES_ZSET + "_user_" + id;
    }

    public ChessState getRoom(String id) {
        expireRooms(GAMES_ZSET);

        String fullId = "game:" + id;
        byte[] bytes = jedis.get(fullId.getBytes());
        if (bytes == null) {
            return null;
        }

        return ChessState.deserialize(bytes);
    }

    public ChessState setRoom(String id, ChessState room) {
        PlayerState whitePlayer = room.getWhitePlayer();
        PlayerState blackPlayer = room.getBlackPlayer();
        room.setTouch(System.currentTimeMillis());

        byte[] bytes = room.serializeAsBytes();
        id = "game:" + id;

        try (AbstractTransaction t = jedis.multi()) {
            t.set(id.getBytes(), bytes);
            t.zadd(GAMES_ZSET, room.getTouch(), id);
            if (whitePlayer != null) {
                t.zadd(getUserGameZSet(whitePlayer.getId()), room.getTouch(), id);
            }
            if (blackPlayer != null) {
                t.zadd(getUserGameZSet(blackPlayer.getId()), room.getTouch(), id);
            }
            t.exec();
        }
        return room;
    }

    public void expireRooms(String zSetName) {
        expireRooms(zSetName, System.currentTimeMillis() - GAME_EXPIRE_FINISHED.toMillis());
    }

    public void expireRooms(String zSetName, long unixTimeExpireMs) {
        List<String> results = jedis.zrangeByScore(zSetName, Double.NEGATIVE_INFINITY, unixTimeExpireMs);
        if (!results.isEmpty()) {
            String[] gameKeys = results.toArray(String[]::new);

            LOG.info("Expiring games with keys={}", Arrays.toString(gameKeys));

            try (AbstractTransaction t = jedis.multi()) {
                t.del(gameKeys);
                t.zrem(GAMES_ZSET, gameKeys);
                t.exec();
            }
        }
    }

    public List<ChessView> getUserChessViews(long userId) {
       return getChessViews(getUserGameZSet(userId), 1, -1);
    }

    public List<ChessView> getChessViews(int page, int count) {
        return getChessViews(GAMES_ZSET, page, count);
    }

    public List<ChessView> getChessViews(String zSetName, int page, int count) {
        expireRooms(zSetName);

        if (page < 1) {
            page = 1;
        }

        int min;
        int max;
        if (count >= 0) {
            min = (page - 1) * count;
            max = min + count - 1;
        } else {
            min = 0;
            max = -1;
        }

        List<byte[]> elements = jedis.zrevrange(zSetName.getBytes(), min, max);

        byte[][] fullIds = new byte[elements.size()][];
        for (int i = 0; i < elements.size(); i++) {
            fullIds[i] = elements.get(i);
        }

        List<byte[]> bytesList = null;
        if (fullIds.length > 0) {
            bytesList = jedis.mget(fullIds);
        }
        if (bytesList == null) {
            bytesList = List.of();
        }

        // handle keys that were inconsistently unset (present in the sorted set but not the dictionary)
        List<byte[]> filteredBytesList = bytesList.stream().filter(Objects::nonNull).toList();
        if (bytesList.size() != filteredBytesList.size()) {
            LOG.warn("Assertion error: expected bytesList len={} and filteredBytesList len={} to be of equal", bytesList.size(), filteredBytesList.size());
        }

        List<ChessView> viewList = filteredBytesList.stream().map(ChessView::deserialize).toList();

        LOG.info("Retrieved chess views={} from set={} for page={}", viewList, zSetName, page);
        return viewList;
    }

    public PlayerState getSession(String sessionId) {
        String fullId = "session:" + sessionId;
        byte[] bytes = jedis.get(fullId.getBytes());
        if (bytes == null) {
            return null;
        }
        return PlayerState.deserialize(bytes);
    }

    public void setSession(String sessionId, PlayerState player, long expirySeconds) {
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

    public PlayerState getSessionOrDefault(String sessionId) {
        PlayerState player;
        if (sessionId != null) {
            player = getSession(sessionId);
        } else {
            String guestName = "Guest " + RANDOM.nextInt(1000);
            long randomLong = Math.abs(RANDOM.nextLong());
            player = new PlayerState(randomLong, guestName, "", 0, true);
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

    public record Leaderboard(List<UserRankEntity> users, int pageCount) {}

    public Leaderboard getLeaderboard(int startRank, int count) {
        List<String> ids = jedis.zrevrange(LEADERBOARD_ZSET, startRank, startRank - 1 + count);
        long elemCount = jedis.zcount(LEADERBOARD_ZSET, Integer.MIN_VALUE, Integer.MAX_VALUE);

        long pageCount = (elemCount / count) + Math.min(elemCount % count, 1);

        List<UserRankEntity> users = new ArrayList<>();
        for (int i = 0; i < ids.size(); i++) {
            long id = Long.parseUnsignedLong(ids.get(i));
            users.add(new UserRankEntity(id, startRank + i + 1));
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

    public void expireUsers(long unixTimeExpireMs) {
        List<String> results = jedis.zrangeByScore(ACTIVE_USERS_ZSET, Double.NEGATIVE_INFINITY, unixTimeExpireMs);
        if (!results.isEmpty()) {
            String[] userKeys = results.toArray(String[]::new);

            LOG.info("Expiring users with keys={}", Arrays.toString(userKeys));
            jedis.zrem(ACTIVE_USERS_ZSET, userKeys);
        }
    }

    public void addUser(String id) {
        jedis.zadd(ACTIVE_USERS_ZSET, System.currentTimeMillis(), id);

        LOG.info("Added user={} into users set", id);
    }

    public void removeUser(String id) {
        jedis.zrem(ACTIVE_USERS_ZSET, id);

        LOG.info("Removed user={} from users set", id);
    }

    public long getUsersCount() {
        expireUsers();
        long count = jedis.zcard(ACTIVE_USERS_ZSET);

        LOG.info("Counted users with result={}", count);
        return count;
    }

    public long getRoomsCount() {
        expireRooms(GAMES_ZSET);
        long count = jedis.zcard(GAMES_ZSET);

        LOG.info("Counted games with result={}", count);
        return count;
    }
}
