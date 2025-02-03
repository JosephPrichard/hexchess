package web;

import com.github.jknack.handlebars.Handlebars;
import com.zaxxer.hikari.HikariDataSource;
import infra.HistoryDao;
import infra.RemoteDict;
import infra.UserDao;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;
import redis.clients.jedis.JedisPooled;
import infra.*;

import java.io.IOException;
import java.util.Map;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class State {
    UserDao userDao;
    HistoryDao historyDao;
    RemoteDict remoteDict;
    GameService gameService;
    SessionService sessionService;
    Broadcaster broadcaster;
    Templates templates;
    Map<String, byte[]> files;

    public State(JedisPooled jedis, HikariDataSource ds, Handlebars handlebars, Map<String, byte[]> filesMap) throws IOException {
        userDao = new UserDao(ds);
        historyDao = new HistoryDao(ds);
        remoteDict = new RemoteDict(jedis);
        gameService = new GameService(remoteDict, userDao, historyDao);
        sessionService = new SessionService();
        broadcaster = new GlobalBroadcaster(jedis);
        templates = new Templates(handlebars);
        files = filesMap;
    }
}