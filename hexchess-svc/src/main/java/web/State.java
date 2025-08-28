package web;

import chess.ChessBoard;
import services.*;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

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