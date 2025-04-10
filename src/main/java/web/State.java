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
    private UserDao userDao;
    private ReplayDao replayDao;
    private ChallengeDao challengeDao;
    private RemoteDict remoteDict;
    private GameService gameService;
    private SessionService sessionService;
    private Broadcaster gameBroadcaster;
    private Broadcaster userBroadcaster;
    private Templates templates;
    private List<String> countryList;
}