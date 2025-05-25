package services.daos;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectReader;
import models.state.ChessRoom;
import models.state.Player;
import models.entities.RankedEntity;
import models.entities.UserEntity;
import redis.clients.jedis.AbstractTransaction;
import redis.clients.jedis.JedisPooled;

import java.io.IOException;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.Random;

import static utils.Globals.*;

public class DictionaryDao {

    private static final Random RANDOM = new Random();
    private final JedisPooled jedis;
    private final ObjectReader playerReader;
    private final ObjectReader roomReader;

    private static final String GAMES_ZSET = "games";
    private static final byte[] GAMES_ZSET_BYTES = GAMES_ZSET.getBytes();
    private static final String LEADERBOARD_ZSET = "leaderboard";
    private static final Duration GAME_EXPIRE_FINISHED = Duration.ofHours(1);
    public static final Duration TEMP_SESSION_EXPIRE = Duration.ofMinutes(1);

    public DictionaryDao(JedisPooled jedis) {
        this.jedis = jedis;
        this.playerReader = MESSAGE_PACK.readerFor(Player.class);
        this.roomReader = MESSAGE_PACK.readerFor(ChessRoom.class);
    }

    private ChessRoom deserializeRoom(byte[] bytes) {
        try {
            return roomReader.readValue(bytes, ChessRoom.class);
        } catch (IOException ex) {
            LOG.error("Failed to deserialize room object from bytes", ex);
            throw new RuntimeException(ex);
        }
    }

    public ChessRoom getRoom(String id) {
        expireRooms();

        String fullId = "game:" + id;
        byte[] bytes = jedis.get(fullId.getBytes());
        if (bytes == null) {
            return null;
        }
        return deserializeRoom(bytes);
    }

    public ChessRoom setRoom(String id, ChessRoom room) {
        room.setTouch(System.currentTimeMillis());

        byte[] bytes;
        try {
           bytes = MESSAGE_PACK.writeValueAsBytes(room);
        } catch (IOException ex) {
            LOG.error("Failed to serialize room object to bytes", ex);
            throw new RuntimeException(ex);
        }

        id = "game:" + id;

        try (AbstractTransaction t = jedis.multi()) {
            t.set(id.getBytes(), bytes);
            t.zadd(GAMES_ZSET, room.getTouch(), id);
            t.exec();
        }

        return room;
    }

    public void expireRooms() {
        expireRooms(GAME_EXPIRE_FINISHED.toMillis());
    }

    public void expireRooms(long expireTimeMillis) {
        long timeMillis = System.currentTimeMillis();
        long unixTimeExpireMillis = timeMillis - expireTimeMillis;
        List<String> results = jedis.zrangeByScore(GAMES_ZSET, Double.NEGATIVE_INFINITY, unixTimeExpireMillis);
        String[] gameKeys = results.toArray(String[]::new);

        if (gameKeys.length > 0) {
            try (AbstractTransaction t = jedis.multi()) {
                t.del(gameKeys);
                t.zrem(GAMES_ZSET, gameKeys);
                t.exec();
            }
        }
    }

    public List<ChessRoom> getRooms(int page, int count) {
        expireRooms();

        if (page < 1) {
            page = 1;
        }

        int min = (page - 1) * count;
        int max = min + count - 1;
        List<byte[]> elements = jedis.zrange(GAMES_ZSET_BYTES, min, max);

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

        return bytesList.stream().map(this::deserializeRoom).toList();
    }

    public Player getSession(String sessionId) {
        String fullId = "session:" + sessionId;
        byte[] bytes = jedis.get(fullId.getBytes());
        if (bytes == null) {
            return null;
        }
        try {
            return playerReader.readValue(bytes, Player.class);
        } catch (IOException ex) {
            LOG.error("Failed to deserialize a player to bytes", ex);
            throw new RuntimeException(ex);
        }
    }

    public void setSession(String sessionId, Player player, long expirySeconds) {
        String fullId = "session:" + sessionId;
        try {
            byte[] bytes = MESSAGE_PACK.writeValueAsBytes(player);
            jedis.setex(fullId.getBytes(), expirySeconds, bytes);
        } catch (JsonProcessingException ex) {
            LOG.error("Failed to serialize a player to bytes", ex);
            throw new RuntimeException(ex);
        }
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
            player = new Player(randomLong, guestName, null, null, true);
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

        LOG.info("Get leaderboard={} of page={}", leaderboard, page);
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
    }

    public void incrLeaderboardUser(long id, double elo) {
        jedis.zincrby(LEADERBOARD_ZSET, elo, Long.toString(id));
    }

    public void updateLeaderboardUser(EloChangeSet... changeSets) {
        try (AbstractTransaction t = jedis.multi()) {
            for (EloChangeSet cs : changeSets) {
                t.zadd(LEADERBOARD_ZSET, cs.elo(), Long.toString(cs.id()));
            }
            t.exec();
        }
    }
}
