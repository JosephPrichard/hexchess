package web;

import services.dao.ChallengeDao;
import services.dao.ReplayDao;
import services.RemoteDict;
import services.dao.UserDao;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;
import services.*;

import java.util.List;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class State {
    public UserDao userDao;
    public ReplayDao replayDao;
    public ChallengeDao challengeDao;
    public RemoteDict remoteDict;
    public GameService gameService;
    public SessionService sessionService;
    public Broadcaster broadcaster;
    public Templates templates;
    public List<String> countryList;
}