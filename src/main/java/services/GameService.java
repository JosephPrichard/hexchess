package services;

import services.dao.ReplayDao;
import services.dao.UserDao;
import domain.ChessGame;
import domain.Move;
import lombok.AllArgsConstructor;
import models.GameState;
import models.ReplayEntity;
import models.PlayerEntity;

import java.util.Random;
import java.util.UUID;

import static services.dao.UserDao.*;
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
    private final ReplayDao replayDao;

    public String create(Boolean isFirstPlayerWhite) {
        String id = UUID.randomUUID().toString();
        GameState gameState = GameState.startWithGame(id);

        gameState.setIsFirstPlayerWhite(isFirstPlayerWhite);
        gameState.getGame().initPieceMoves();

        remoteDict.setGame(id, gameState);
        return id;
    }

    public GameState join(String gameId, PlayerEntity player) {
        GameState state = remoteDict.getGame(gameId);
        if (state == null) {
            return null;
        }

        boolean hasWhitePlayer = state.getWhitePlayer() != null;
        boolean hasBlackPlayer = state.getBlackPlayer() != null;

        boolean joinedAsWhite;
        if (!hasWhitePlayer && !hasBlackPlayer) {
            // neither player, so join as either
            Boolean isFirstPlayerWhite = state.getIsFirstPlayerWhite();
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

    public GameState makeMove(String gameId, PlayerEntity player, Move move) {
        GameState state = remoteDict.getGame(gameId);
        if (state == null) {
            return null;
        }

        ChessGame game = state.getGame();

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

        state.pushMoveList(move);

        if (game.isCheckmate()) {
            state.setEnded(true);
            boolean isWhiteWin = game.getBoard().turn().isBlack(); // white wins if its checkmate when it's blacks turn
            EXECUTOR.execute(() -> onFinishGame(state, isWhiteWin));
        }

        LOGGER.info("{} made move {} on game {}", player, move, gameId);
        return remoteDict.setGame(gameId, state);
    }

    public void onFinishGame(GameState state, boolean isWhiteWin) {
        try {
            long whiteId = state.getWhitePlayer().getId();
            long blackId = state.getBlackPlayer().getId();
            int result = isWhiteWin ? ReplayEntity.WHITE_WIN : ReplayEntity.BLACK_WIN;
            long winId = isWhiteWin ? whiteId : blackId;
            long loseId = isWhiteWin ? blackId : whiteId;

            String moveListJson = JSON_MAPPER.writeValueAsString(state.getMoveList());

            EloChangeSet changeSet = userDao.updateStats(winId, loseId);
            remoteDict.incrLeaderboardUser(
                    new RemoteDict.EloChangeSet(winId, changeSet.getWinEloDiff()),
                    new RemoteDict.EloChangeSet(loseId, changeSet.getLoseEloDiff()));
            replayDao.insert(whiteId, blackId, result, changeSet.getWinEloDiff(), changeSet.getLoseEloDiff(), moveListJson);
        } catch (Exception ex) {
            LOGGER.info("Failed to persist game results to database in background thread {}", String.valueOf(ex));
        }
    }

    public GameState forfeit(String gameId, PlayerEntity player) {
        GameState state = remoteDict.getGame(gameId);
        if (state == null) {
            return null;
        }

        boolean didBlackForfeit = state.getBlackPlayer().equals(player);

        state.setEnded(true);
        onFinishGame(state, didBlackForfeit);

        return remoteDict.setGame(gameId, state); // did black forfeit? then white won.
    }

    public RemoteDict.GetGamesResult getGames(Double cursor) {
        return remoteDict.getGames(cursor, 20);
    }
}
