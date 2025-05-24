package web;

import chess.ChessBoard;
import services.broadcast.Broadcaster;
import services.daos.DictionaryDao;
import services.daos.ChallengeDao;
import services.daos.ReplayDao;
import services.game.GameService;
import services.daos.UserDao;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;
import services.producers.ChallengeProducer;
import web.reusable.AuthService;
import web.reusable.PathService;

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
    private PathService pathService;
    private Broadcaster gameBroadcaster;
    private Broadcaster userBroadcaster;
    private ChallengeProducer challengeProducer;

    private List<String> countryList;
    private ChessBoard initialBoard;
}