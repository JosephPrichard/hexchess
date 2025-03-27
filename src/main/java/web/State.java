package web;

import com.github.jknack.handlebars.Handlebars;
import com.zaxxer.hikari.HikariDataSource;
import dao.ChallengeDao;
import dao.HistoryDao;
import services.RemoteDict;
import dao.UserDao;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;
import redis.clients.jedis.JedisPooled;
import services.*;

import java.io.IOException;
import java.util.Map;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class State {
    public UserDao userDao;
    public HistoryDao historyDao;
    public ChallengeDao challengeDao;
    public RemoteDict remoteDict;
    public GameService gameService;
    public SessionService sessionService;
    public Broadcaster broadcaster;
    public Templates templates;
    public Map<String, byte[]> files;

    public State(JedisPooled jedis, HikariDataSource ds, Handlebars handlebars, Map<String, byte[]> filesMap) throws IOException {
        userDao = new UserDao(ds);
        historyDao = new HistoryDao(ds);
        challengeDao = new ChallengeDao(ds);
        remoteDict = new RemoteDict(jedis);
        gameService = new GameService(remoteDict, userDao, historyDao);
        sessionService = new SessionService();
        broadcaster = new GlobalBroadcaster(jedis);
        templates = new Templates(handlebars);
        files = filesMap;
    }
}