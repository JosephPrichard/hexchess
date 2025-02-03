package services;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectReader;
import lombok.AllArgsConstructor;
import lombok.Data;
import models.GameState;
import models.Player;
import models.RankedUser;
import redis.clients.jedis.JedisPooled;
import utils.Serializer;

import java.io.IOException;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.Random;
import java.util.UUID;

import static utils.Globals.JSON_MAPPER;
import static utils.Globals.LOGGER;

public class RemoteDict {

    private static final Random RANDOM = new Random();
    private final JedisPooled jedis;
    private final ObjectReader playerReader;

    private static final String GAMES_ZSET = "games";
    private static final String LEADERBOARD_ZSET = "leaderboard";
    private static final Duration GAME_EXPIRE_FINISHED = Duration.ofHours(1);

    public RemoteDict(JedisPooled jedis) {
        this.jedis = jedis;
        this.playerReader = JSON_MAPPER.readerFor(Player.class);
    }

    public GameState getGame(String id) {
        expireGames();

        var fullId = "game:" + id;
        var bytes = jedis.get(fullId.getBytes());
        if (bytes == null) {
            return null;
        }
        return Serializer.deserialize(bytes, GameState.class);
    }

    public GameState setGame(String id, GameState gameState) {
        double timeNanos = System.nanoTime();
        gameState.setTouch(timeNanos);

        var bytes = Serializer.serialize(gameState);
        var fullId = "game:" + id;

        var t = jedis.multi();
        t.set(fullId.getBytes(), bytes);
        t.zadd(GAMES_ZSET, timeNanos, fullId);
        t.exec();

        return gameState;
    }

    public void expireGames() {
        expireGames(GAME_EXPIRE_FINISHED.toNanos());
    }

    public void expireGames(long expireTimeNanos) {
        long timeNanos = System.nanoTime();
        long unixTimeExpireNanos = timeNanos - expireTimeNanos;
        var results = jedis.zrangeByScore(GAMES_ZSET, Double.NEGATIVE_INFINITY, unixTimeExpireNanos);
        var gameKeys = results.toArray(String[]::new);

        if (gameKeys.length > 0) {
            var t = jedis.multi();
            t.del(gameKeys);
            t.zrem(GAMES_ZSET, gameKeys);
            t.exec();
        }
    }

    @Data
    @AllArgsConstructor
    public static class GetGamesResult {
        Double nextCursor;
        List<GameState> gameStates;
    }

    public GetGamesResult getGames(Double cursor, int count) {
        expireGames();

        cursor = cursor != null ? cursor : 0;
        var tuples = jedis.zrangeByScoreWithScores(GAMES_ZSET, cursor, Double.POSITIVE_INFINITY, 0, count + 1);

        // discard the last element, if we know for sure we over fetched, and use it as the next cursor
        Double nextCursor = null;
        if (tuples.size() >= count + 1) {
            nextCursor = tuples.removeLast().getScore();
        }

        byte[][] fullIds = new byte[tuples.size()][];
        for (int i = 0; i < tuples.size(); i++) {
            fullIds[i] = tuples.get(i).getBinaryElement();
        }

        var bytesList = jedis.mget(fullIds);
        if (bytesList == null) {
            return null;
        }

        var gameStates = bytesList.stream().map((bytes) -> Serializer.deserialize(bytes, GameState.class)).toList();
        return new GetGamesResult(nextCursor, gameStates);
    }

    public Player getSession(String sessionId) {
        var fullId = "session:" + sessionId;
        var str = jedis.get(fullId);
        if (str == null) {
            return null;
        }
        try {
            return playerReader.readValue(str, Player.class);
        } catch (IOException ex) {
            LOGGER.info("Failed to parse json object from the dictionary {}", String.valueOf(ex));
            return null;
        }
    }

    public void setSession(String sessionId, Player player, long expirySeconds) {
        var fullId = "session:" + sessionId;
        try {
            var str = JSON_MAPPER.writeValueAsString(player);
            jedis.setex(fullId, expirySeconds, str);
        } catch (JsonProcessingException ex) {
            LOGGER.info("Failed to serialize an input json object to dictionary {}", String.valueOf(ex));
        }
    }

    public void updateSessionEx(String sessionId, long expirySeconds) {
        var fullId = "session:" + sessionId;
        jedis.expire(fullId, expirySeconds);
    }

    public void deleteSession(String sessionId) {
        var fullId = "session:" + sessionId;
        jedis.del(fullId);
    }

    public Player getSessionOrDefault(String sessionId) {
        Player player;
        if (sessionId != null) {
            player = getSession(sessionId);
        } else {
            var guestName = "Guest " + RANDOM.nextInt(1000);
            player = new Player(UUID.randomUUID().toString(), guestName);
        }
        return player;
    }

    public int getLeaderboardRank(String id) {
        return jedis.zrank(LEADERBOARD_ZSET, id).intValue() + 1;
    }

    @Data
    @AllArgsConstructor
    public static class Leaderboard {
        List<RankedUser> users;
        int pageCount;
    }

    public Leaderboard getLeaderboard(int startRank, int count) {
        var ids = jedis.zrange(LEADERBOARD_ZSET, startRank, startRank - 1 + count);
        var elemCount = jedis.zcount(LEADERBOARD_ZSET, Integer.MIN_VALUE, Integer.MAX_VALUE);

        var pageCount = elemCount / count;

        List<RankedUser> users = new ArrayList<>();
        for (int i = 0; i < ids.size(); i++) {
            var id = ids.get(i);
            users.add(new RankedUser(id, startRank + i + 1));
        }
        return new Leaderboard(users, (int) pageCount);
    }

    public Leaderboard getLeaderboardPage(int page, int perPage) {
        page = Math.max(page, 1);
        var offset = (page - 1) * perPage;
        return getLeaderboard(offset, perPage);
    }

    @Data
    @AllArgsConstructor
    public static class EloChangeSet {
        String id;
        double elo;
    }

    public void incrLeaderboardUser(EloChangeSet... changeSets) {
        var t = jedis.multi();
        for (var cs : changeSets) {
            t.zincrby(LEADERBOARD_ZSET, cs.elo, cs.id);
        }
        t.exec();
    }

    public void incrLeaderboardUser(String id, double elo) {
        jedis.zincrby(LEADERBOARD_ZSET, elo, id);
    }

    public void updateLeaderboardUser(EloChangeSet... changeSets) {
        var t = jedis.multi();
        for (var cs : changeSets) {
            t.zadd(LEADERBOARD_ZSET, cs.elo, cs.id);
        }
        t.exec();
    }
}
