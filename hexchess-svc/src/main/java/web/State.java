package web;

import chess.ChessBoard;
import services.broadcast.Broadcaster;
import services.broadcast.SingleBroadcaster;
import services.daos.DictionaryDao;
import services.daos.ChallengeDao;
import services.daos.ReplayDao;
import services.game.GameService;
import services.daos.UserDao;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;
import web.reusable.AuthService;

import java.util.List;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class State {
    private UserDao userDao;
    private ReplayDao replayDao;
    private ChallengeDao challengeDao;
    private DictionaryDao dictionaryDao;
    private GameService gameService;
    private AuthService authService;
    private Broadcaster gameBroadcaster;
    private Broadcaster userBroadcaster;
    private SingleBroadcaster userCountBroadcaster;
    private SingleBroadcaster gameCountBroadcaster;

    private List<String> countryList;
    private ChessBoard initialBoard;
}