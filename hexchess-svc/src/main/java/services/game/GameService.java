package services.game;

import chess.ChessBoard;
import chess.PieceMove;
import services.broadcast.SingleBroadcaster;
import services.daos.DictionaryDao;
import services.daos.ReplayDao;
import services.daos.UserDao;
import chess.ChessGame;
import lombok.AllArgsConstructor;
import models.common.ColorSelect;
import models.common.TimeControl;
import models.state.Player;
import models.entities.ReplayEntity;
import models.state.ChessRoom;

import java.util.concurrent.CompletableFuture;

import static utils.Globals.*;

@AllArgsConstructor
public class GameService {

    public static class FinishedGameException extends RuntimeException {}

    public static class TurnException extends RuntimeException {}

    public static class InvalidMoveException extends RuntimeException {}

    private final DictionaryDao dictionaryDao;
    private final UserDao userDao;
    private final ReplayDao replayDao;
    private final SingleBroadcaster gameCountBroadcaster;

    public static String generateGameId() {
        int length = 8;
        StringBuilder sb = new StringBuilder(length);
        for (int i = 0; i < length; i++) {
            int index = SECURE_RANDOM.nextInt(CHARACTERS.length());
            sb.append(CHARACTERS.charAt(index));
        }
        return sb.toString();
    }

    public String create(ColorSelect color, TimeControl timeControl) {
        String id = generateGameId();
        ChessRoom room = ChessRoom.startWithGame(id, timeControl);

        room.setFirstColor(color);
        room.getGame().initPieceMoves();

        LOG.info("Created chess game with room={}", room);
        dictionaryDao.setRoom(id, room);

        CompletableFuture.runAsync(() -> gameCountBroadcaster.broadcast(Long.toString(dictionaryDao.getRoomsCount())), EXECUTOR);

        return id;
    }

    public ChessRoom join(String gameId, Player player) {
        ChessRoom room = dictionaryDao.getRoom(gameId);
        if (room == null) {
            return null;
        }

        boolean hasWhite = room.getWhitePlayer() != null;
        boolean hasBlack = room.getBlackPlayer() != null;
        boolean playerExists = hasWhite && player.equals(room.getWhitePlayer()) || hasBlack && player.equals(room.getBlackPlayer());

        String color = "none";
        if (!playerExists) {
            if (!hasWhite && !hasBlack) {
                boolean chooseWhite = room.getFirstColor() == ColorSelect.RANDOM ? RANDOM.nextInt() % 2 == 0 : room.getFirstColor() == ColorSelect.WHITE;
                if (chooseWhite) {
                    room.setWhitePlayer(player);
                    color = "white";
                } else {
                    room.setBlackPlayer(player);
                    color = "black";
                }
            } else if (!hasBlack) {
                room.setBlackPlayer(player);
                color = "black";
            } else if (!hasWhite) {
                room.setWhitePlayer(player);
                color = "white";
            } else {
                return room;
            }
        }

        LOG.info("Player={} joined the game={} as color={}", player.getId(), gameId, color);

        room = dictionaryDao.setRoom(gameId, room);
        return room;
    }

    public record MakeMoveResult(ChessRoom room, PieceMove move) {}

    public MakeMoveResult makeMove(String gameId, Player player, PieceMove move) {
        ChessRoom room = dictionaryDao.getRoom(gameId);
        if (room == null) {
            return null;
        }

        ChessGame game = room.getGame();
        ChessBoard board = game.getBoard();

        Player currPlayer = room.getCurrPlayer();
        boolean isPlayerTurn = currPlayer != null && currPlayer.equals(player);

        if (room.isEnded()) {
            LOG.info("Move attempted on ended game {}", gameId);
            throw new FinishedGameException();
        }
        if (!isPlayerTurn) {
            LOG.info("{} cannot make move on game {}, it isn't their turn", player, gameId);
            throw new TurnException();
        }
        if (!game.isValidMove(move)) {
            LOG.info(" {} made invalid move {} on game {}", player, move, gameId);
            throw new InvalidMoveException();
        }

        move = game.makeMove(move);
        game.initPieceMoves();

        room.addMove(move);

        if (game.checkmateReached()) {
            room.setEnded(true);
            boolean isWhiteWin = !board.isWhiteTurn(); // white wins if its checkmate when it's blacks turn
            CompletableFuture.runAsync(() -> onFinishGame(room, isWhiteWin, ReplayEntity.CHECKMATE), EXECUTOR);
        }

        LOG.info("{} made move {} on game {}", player, move, gameId);
        return new MakeMoveResult(dictionaryDao.setRoom(gameId, room), move);
    }

    public void onFinishGame(ChessRoom room, boolean isWhiteWin, int cause) {
        try {
            assert room.getWhitePlayer() != null;
            assert room.getBlackPlayer() != null;

            long whiteId = room.getWhitePlayer().getId();
            long blackId = room.getBlackPlayer().getId();
            int result = isWhiteWin ? ReplayEntity.WHITE_WIN : ReplayEntity.BLACK_WIN;
            long winId = isWhiteWin ? whiteId : blackId;
            long loseId = isWhiteWin ? blackId : whiteId;

            String moveListJson = JSON.writeValueAsString(room.getMoveList());
            UserDao.EloChangeSet changeSet = userDao.updateStats(winId, loseId);
            if (changeSet == null) {
                LOG.info("No change set needs to be applied to game result for room={}", room.getId());
                return;
            }

            LOG.info("Applying changeSet={} on room={}", changeSet, room.getId());

            dictionaryDao.incrLeaderboardUser(
                new DictionaryDao.EloChangeSet(winId, changeSet.winEloDiff()),
                new DictionaryDao.EloChangeSet(loseId, changeSet.loseEloDiff()));
            replayDao.insert(whiteId, blackId, result, cause, changeSet.winEloDiff(), changeSet.loseEloDiff(), moveListJson);
        } catch (Exception ex) {
            LOG.info("Failed to persist game results to database in background thread {}", String.valueOf(ex));
        }
    }

    public void forfeit(String gameId, Player player) {
        ChessRoom state = dictionaryDao.getRoom(gameId);
        if (state == null || state.getBlackPlayer() == null || state.getWhitePlayer() == null) {
            return;
        }

        boolean didBlackForfeit = state.getBlackPlayer().equals(player);

        state.setEnded(true);
        onFinishGame(state, didBlackForfeit, ReplayEntity.FORFEIT); // did black forfeit? then white won.

        dictionaryDao.setRoom(gameId, state);
    }
}
