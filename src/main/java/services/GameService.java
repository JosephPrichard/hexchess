package services;

import dao.HistoryDao;
import dao.UserDao;
import domain.Move;
import lombok.AllArgsConstructor;
import models.GameState;
import models.History;
import models.Player;

import java.util.Random;
import java.util.UUID;

import static utils.Globals.*;

@AllArgsConstructor
public class GameService {

    public static class MoveException extends RuntimeException {
        public MoveException(String message) {
            super(message);
        }
    }

    private static final Random RANDOM = new Random();

    private final RemoteDict remoteDict;
    private final UserDao userDao;
    private final HistoryDao historyDao;

    public String create(Boolean isFirstPlayerWhite) {
        var id = UUID.randomUUID().toString();
        var gameState = GameState.startWithGame(id);

        gameState.setIsFirstPlayerWhite(isFirstPlayerWhite);
        gameState.getGame().initPieceMoves();

        remoteDict.setGame(id, gameState);
        return id;
    }

    public GameState join(String gameId, Player player) {
        var state = remoteDict.getGame(gameId);
        if (state == null) {
            return null;
        }

        var hasWhitePlayer = state.getWhitePlayer() != null;
        var hasBlackPlayer = state.getBlackPlayer() != null;

        boolean joinedAsWhite;
        if (!hasWhitePlayer && !hasBlackPlayer) {
            // neither player, so join as either
            var isFirstPlayerWhite = state.getIsFirstPlayerWhite();
            boolean chooseWhite = isFirstPlayerWhite == null ? RANDOM.nextInt() % 2 == 0 : isFirstPlayerWhite;
            if (chooseWhite) {
                state.setWhitePlayer(player);
                joinedAsWhite = true;
            } else {
                state.setBlackPlayer(player);
                joinedAsWhite = false;
            }
        } else if (!hasBlackPlayer) {
            // no black player, so join as black
            state.setBlackPlayer(player);
            joinedAsWhite = false;
        } else if (!hasWhitePlayer) {
            // no white player, so join as white
            state.setWhitePlayer(player);
            joinedAsWhite = true;
        } else {
            // both players, so we cannot join... just return the game data to view
            return state;
        }

        if (joinedAsWhite) {
            LOGGER.info("Player {} joined as white player {}", player.getId(), gameId);
        } else {
            LOGGER.info("Player {} joined as black player {}", player.getId(), gameId);
        }

        return remoteDict.setGame(gameId, state);
    }

    public GameState makeMove(String gameId, Player player, Move move) {
        var state = remoteDict.getGame(gameId);
        if (state == null) {
            return null;
        }

        var game = state.getGame();

        if (state.isEnded()) {
            LOGGER.info("Move attempted on ended game {}", gameId);
            throw new MoveException("Cannot make a move on a game that is over!");
        }
        if (!state.isPlayerTurn(player)) {
            LOGGER.info("{} cannot make move on game {}, it isn't their turn", player, gameId);
            throw new MoveException("Cannot make a move when it isn't your turn!");
        }
        if (!game.isValidMove(move)) {
            LOGGER.info(" {} made invalid move {} on game {}", player, move, gameId);
            throw new MoveException("Cannot make an invalid move!");
        }

        game.makeMove(move);
        game.initPieceMoves();

        state.pushMoveHistory(move);

        if (game.isCheckmate()) {
            state.setEnded(true);
            var isWhiteWin = game.getBoard().turn().isBlack(); // white wins if its checkmate when it's blacks turn
            EXECUTOR.execute(() -> onFinishGame(state, isWhiteWin));
        }

        LOGGER.info("{} made move {} on game {}", player, move, gameId);
        return remoteDict.setGame(gameId, state);
    }

    public void onFinishGame(GameState state, boolean isWhiteWin) {
        try {
            var whiteId = state.getWhitePlayer().getId();
            var blackId = state.getBlackPlayer().getId();
            var result = isWhiteWin ? History.WHITE_WIN : History.BLACK_WIN;
            var winId = isWhiteWin ? whiteId : blackId;
            var loseId = isWhiteWin ? blackId : whiteId;

            var moveHistoryData = JSON_MAPPER.writeValueAsString(state.getMoveList());

            var changeSet = userDao.updateStats(winId, loseId);
//            remoteDict.updateLeaderboardUser(
//                new RemoteDict.EloChangeSet(winId, changeSet.winEloDiff),
//                new RemoteDict.EloChangeSet(loseId, changeSet.loseEloDiff));
//            historyDao.insert(whiteId, blackId, result, changeSet.getWinEloDiff(), changeSet.getLoseEloDiff(), moveHistoryData);
        } catch (Exception ex) {
            LOGGER.info("Failed to persist game results to database in background thread {}", String.valueOf(ex));
        }
    }

    public GameState forfeit(String gameId, Player player) {
        var state = remoteDict.getGame(gameId);
        if (state == null) {
            return null;
        }

        var didBlackForfeit = state.getBlackPlayer().equals(player);

        state.setEnded(true);
        onFinishGame(state, didBlackForfeit);

        return remoteDict.setGame(gameId, state); // did black forfeit? then white won.
    }

    public RemoteDict.GetGamesResult getGames(Double cursor) {
        return remoteDict.getGames(cursor, 20);
    }
}
