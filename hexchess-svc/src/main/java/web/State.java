package web;

import chess.ChessBoard;
import services.broadcast.GroupBroadcaster;
import services.broadcast.SingleBroadcaster;
import services.daos.*;
import services.game.GameService;
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
    private GroupBroadcaster gameBroadcaster;
    private GroupBroadcaster userBroadcaster;
    private SingleBroadcaster userCountBroadcaster;
    private SingleBroadcaster gameCountBroadcaster;

    private List<String> countryList;
    private ChessBoard initialBoard;
}