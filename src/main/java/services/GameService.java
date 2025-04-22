package services;

import chess.PieceMove;
import daos.ReplayDao;
import daos.UserDao;
import chess.ChessGame;
import chess.Move;
import lombok.AllArgsConstructor;
import models.GameState;
import models.ReplayEntity;
import models.PlayerEntity;

import java.util.List;
import java.util.Random;
import java.util.UUID;

import static daos.UserDao.*;
import static utils.Globals.*;

@AllArgsConstructor
public class GameService {

    public static class FinishedGameException extends RuntimeException {}

    public static class TurnException extends RuntimeException {}

    public static class InvalidMoveException extends RuntimeException {}

    private static final Random RANDOM = new Random();

    private final RemoteDict remoteDict;
    private final UserDao userDao;
    private final ReplayDao replayDao;

    public String create(Boolean isFirstPlayerWhite) {
        String id = UUID.randomUUID().toString();
        GameState gameState = GameState.startWithGame(id);

        gameState.isFirstPlayerWhite = isFirstPlayerWhite;
        gameState.game.initPieceMoves();

        remoteDict.setGame(id, gameState);
        return id;
    }

    public GameState join(String gameId, PlayerEntity player) {
        GameState state = remoteDict.getGame(gameId);
        if (state == null) {
            return null;
        }

        boolean hasWhitePlayer = state.whitePlayer != null;
        boolean hasBlackPlayer = state.blackPlayer != null;

        boolean joinedAsWhite;
        if (!hasWhitePlayer && !hasBlackPlayer) {
            boolean chooseWhite = state.isFirstPlayerWhite == null ? RANDOM.nextInt() % 2 == 0 : state.isFirstPlayerWhite;
            if (chooseWhite) {
                state.whitePlayer = player;
                joinedAsWhite = true;
            } else {
                state.blackPlayer = player;
                joinedAsWhite = false;
            }
        } else if (!hasBlackPlayer) {
            state.blackPlayer = player;
            joinedAsWhite = false;
        } else if (!hasWhitePlayer) {
            state.whitePlayer = player;
            joinedAsWhite = true;
        } else {
            return state;
        }

        if (joinedAsWhite) {
            LOGGER.info("Player {} joined as white player {}", player.id, gameId);
        } else {
            LOGGER.info("Player {} joined as black player {}", player.id, gameId);
        }

        return remoteDict.setGame(gameId, state);
    }

    public GameState makeMove(String gameId, PlayerEntity player, Move move) {
        GameState state = remoteDict.getGame(gameId);
        if (state == null) {
            return null;
        }

        ChessGame game = state.game;

        if (state.isEnded) {
            LOGGER.info("Move attempted on ended game {}", gameId);
            throw new FinishedGameException();
        }
        if (!state.isPlayerTurn(player)) {
            LOGGER.info("{} cannot make move on game {}, it isn't their turn", player, gameId);
            throw new TurnException();
        }
        if (!game.isValidMove(move)) {
            LOGGER.info(" {} made invalid move {} on game {}", player, move, gameId);
            throw new InvalidMoveException();
        }

        byte piece = game.getBoard().getPiece(move.getFrom());
        game.makeMove(move);
        game.initPieceMoves();

        state.pushMoveList(new PieceMove(piece, move.getFrom(), move.getTo()));

        if (game.isCheckmate()) {
            state.isEnded = true;
            boolean isWhiteWin = game.getBoard().turn().isBlack(); // white wins if its checkmate when it's blacks turn
            EXECUTOR.execute(() -> onFinishGame(state, isWhiteWin, ReplayEntity.CHECKMATE));
        }

        LOGGER.info("{} made move {} on game {}", player, move, gameId);
        return remoteDict.setGame(gameId, state);
    }

    public void onFinishGame(GameState state, boolean isWhiteWin, int cause) {
        try {
            long whiteId = state.whitePlayer.id;
            long blackId = state.blackPlayer.id;
            int result = isWhiteWin ? ReplayEntity.WHITE_WIN : ReplayEntity.BLACK_WIN;
            long winId = isWhiteWin ? whiteId : blackId;
            long loseId = isWhiteWin ? blackId : whiteId;

            String moveListJson = JSON_MAPPER.writeValueAsString(state.moveList);

            EloChangeSet changeSet = userDao.updateStats(winId, loseId);
            remoteDict.incrLeaderboardUser(
                    new RemoteDict.EloChangeSet(winId, changeSet.winEloDiff),
                    new RemoteDict.EloChangeSet(loseId, changeSet.loseEloDiff));
            replayDao.insert(whiteId, blackId, result, cause, changeSet.winEloDiff, changeSet.loseEloDiff, moveListJson);
        } catch (Exception ex) {
            LOGGER.info("Failed to persist game results to database in background thread {}", String.valueOf(ex));
        }
    }

    public GameState forfeit(String gameId, PlayerEntity player) {
        GameState state = remoteDict.getGame(gameId);
        if (state == null) {
            return null;
        }

        boolean didBlackForfeit = state.blackPlayer.equals(player);

        state.isEnded = true;
        onFinishGame(state, didBlackForfeit, ReplayEntity.FORFEIT);

        return remoteDict.setGame(gameId, state); // did black forfeit? then white won.
    }

    public List<GameState> getGames(int page) {
        return remoteDict.getGames(page, 20);
    }
}
