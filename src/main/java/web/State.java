package web;

import services.Broadcaster;
import daos.ChallengeDao;
import daos.ReplayDao;
import services.GameService;
import services.RemoteDict;
import daos.UserDao;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;
import services.SessionService;

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