package services.game;

import chess.PieceMove;
import services.daos.DictionaryDao;
import services.daos.ReplayDao;
import services.daos.UserDao;
import chess.ChessGame;
import chess.Move;
import lombok.AllArgsConstructor;
import models.common.ColorSelect;
import models.common.TimeControl;
import models.entities.PlayerEntity;
import models.entities.ReplayEntity;
import models.state.GameState;

import java.util.List;

import static services.daos.UserDao.*;
import static utils.Globals.*;

@AllArgsConstructor
public class GameService {

    public static class FinishedGameException extends RuntimeException {}

    public static class TurnException extends RuntimeException {}

    public static class InvalidMoveException extends RuntimeException {}

    private final DictionaryDao dictionaryDao;
    private final UserDao userDao;
    private final ReplayDao replayDao;

    public static String generateGameId() {
        int length = 8;
        StringBuilder sb = new StringBuilder(length);
        for (int i = 0; i < length; i++) {
            int index = SECURE_RANDOM.nextInt(CHARACTERS.length());
            sb.append(CHARACTERS.charAt(index));
        }
        return sb.toString();
    }

    public String create(ColorSelect order, TimeControl timeControl) {
        String id = generateGameId();
        GameState gameState = GameState.startWithGame(id, timeControl);

        gameState.firstColor = order;
        gameState.game.initPieceMoves();

        LOGGER.info("Created game={}", gameState);

        dictionaryDao.setGame(id, gameState);
        return id;
    }

    public GameState join(String gameId, PlayerEntity player) {
        GameState state = dictionaryDao.getGame(gameId);
        if (state == null) {
            return null;
        }

        boolean hasWhitePlayer = state.whitePlayer != null;
        boolean hasBlackPlayer = state.blackPlayer != null;
        boolean hasNoPlayers = !hasWhitePlayer && !hasBlackPlayer;
        boolean playerExists = hasWhitePlayer && player.equals(state.whitePlayer) || hasBlackPlayer && player.equals(state.blackPlayer);

        if (!playerExists) {
            boolean joinedAsWhite;
            if (hasNoPlayers) {
                boolean chooseWhite = state.firstColor == ColorSelect.RANDOM ?
                        RANDOM.nextInt() % 2 == 0 :
                        state.firstColor == ColorSelect.WHITE;
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
        }

        return dictionaryDao.setGame(gameId, state);
    }

    public static boolean isPlayerTurn(GameState state, PlayerEntity player) {
        PlayerEntity currPlayer = state.getCurrPlayer();
        if (currPlayer == null) {
            return false;
        }
        return currPlayer.equals(player);
    }

    public GameState makeMove(String gameId, PlayerEntity player, Move move) {
        GameState state = dictionaryDao.getGame(gameId);
        if (state == null) {
            return null;
        }

        ChessGame game = state.game;

        if (state.isEnded) {
            LOGGER.info("Move attempted on ended game {}", gameId);
            throw new FinishedGameException();
        }
        if (!isPlayerTurn(state, player)) {
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

        state.moveList.add(new PieceMove(piece, move.getFrom(), move.getTo()));

        if (game.isCheckmate()) {
            state.isEnded = true;
            boolean isWhiteWin = game.getBoard().turn().isBlack(); // white wins if its checkmate when it's blacks turn
            EXECUTOR.execute(() -> onFinishGame(state, isWhiteWin, ReplayEntity.CHECKMATE));
        }

        LOGGER.info("{} made move {} on game {}", player, move, gameId);
        return dictionaryDao.setGame(gameId, state);
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
            dictionaryDao.incrLeaderboardUser(
                    new DictionaryDao.EloChangeSet(winId, changeSet.winEloDiff),
                    new DictionaryDao.EloChangeSet(loseId, changeSet.loseEloDiff));
            replayDao.insert(whiteId, blackId, result, cause, changeSet.winEloDiff, changeSet.loseEloDiff, moveListJson);
        } catch (Exception ex) {
            LOGGER.info("Failed to persist game results to database in background thread {}", String.valueOf(ex));
        }
    }

    public GameState forfeit(String gameId, PlayerEntity player) {
        GameState state = dictionaryDao.getGame(gameId);
        if (state == null) {
            return null;
        }

        boolean didBlackForfeit = state.blackPlayer.equals(player);

        state.isEnded = true;
        onFinishGame(state, didBlackForfeit, ReplayEntity.FORFEIT);

        return dictionaryDao.setGame(gameId, state); // did black forfeit? then white won.
    }

    public List<GameState> getGames(int page) {
        return dictionaryDao.getGames(page, 20);
    }
}
