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
import models.state.ChessRoom;

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

    public String create(ColorSelect color, TimeControl timeControl) {
        String id = generateGameId();
        ChessRoom chessRoom = ChessRoom.startWithGame(id, timeControl);

        chessRoom.setFirstColor(color);
        chessRoom.getGame().initPieceMoves();

        LOG.info("Created game={}", chessRoom);

        dictionaryDao.setGame(id, chessRoom);
        return id;
    }

    public ChessRoom join(String gameId, PlayerEntity player) {
        ChessRoom room = dictionaryDao.getGame(gameId);
        if (room == null) {
            return null;
        }

        boolean hasWhitePlayer = room.getWhitePlayer() != null;
        boolean hasBlackPlayer = room.getBlackPlayer() != null;
        boolean hasNoPlayers = !hasWhitePlayer && !hasBlackPlayer;
        boolean playerExists = hasWhitePlayer && player.equals(room.getWhitePlayer()) || hasBlackPlayer && player.equals(room.getBlackPlayer());

        if (!playerExists) {
            boolean joinedAsWhite;
            if (hasNoPlayers) {
                boolean chooseWhite = room.getFirstColor() == ColorSelect.RANDOM ?
                        RANDOM.nextInt() % 2 == 0 :
                        room.getFirstColor() == ColorSelect.WHITE;
                if (chooseWhite) {
                    room.setWhitePlayer(player);
                    joinedAsWhite = true;
                } else {
                    room.setBlackPlayer(player);
                    joinedAsWhite = false;
                }
            } else if (!hasBlackPlayer) {
                room.setBlackPlayer(player);
                joinedAsWhite = false;
            } else if (!hasWhitePlayer) {
                room.setWhitePlayer(player);
                joinedAsWhite = true;
            } else {
                return room;
            }

            if (joinedAsWhite) {
                LOG.info("Player {} joined as white player {}", player.getId(), gameId);
            } else {
                LOG.info("Player {} joined as black player {}", player.getId(), gameId);
            }
        }

        return dictionaryDao.setGame(gameId, room);
    }

    public static boolean isPlayerTurn(ChessRoom state, PlayerEntity player) {
        PlayerEntity currPlayer = state.getCurrPlayer();
        if (currPlayer == null) {
            return false;
        }
        return currPlayer.equals(player);
    }

    public ChessRoom makeMove(String gameId, PlayerEntity player, Move move) {
        ChessRoom room = dictionaryDao.getGame(gameId);
        if (room == null) {
            return null;
        }

        ChessGame game = room.getGame();

        if (room.isEnded()) {
            LOG.info("Move attempted on ended game {}", gameId);
            throw new FinishedGameException();
        }
        if (!isPlayerTurn(room, player)) {
            LOG.info("{} cannot make move on game {}, it isn't their turn", player, gameId);
            throw new TurnException();
        }
        if (!game.isValidMove(move)) {
            LOG.info(" {} made invalid move {} on game {}", player, move, gameId);
            throw new InvalidMoveException();
        }

        byte piece = game.getBoard().getPiece(move.getFrom());
        game.makeMove(move);
        game.initPieceMoves();

        room.getMoveList().add(new PieceMove(piece, move.getFrom(), move.getTo()));

        if (game.isCheckmate()) {
            room.setEnded(true);
            boolean isWhiteWin = game.getBoard().turn().isBlack(); // white wins if its checkmate when it's blacks turn
            EXECUTOR.execute(() -> onFinishGame(room, isWhiteWin, ReplayEntity.CHECKMATE));
        }

        LOG.info("{} made move {} on game {}", player, move, gameId);
        return dictionaryDao.setGame(gameId, room);
    }

    public void onFinishGame(ChessRoom state, boolean isWhiteWin, int cause) {
        try {
            long whiteId = state.getWhitePlayer().getId();
            long blackId = state.getBlackPlayer().getId();
            int result = isWhiteWin ? ReplayEntity.WHITE_WIN : ReplayEntity.BLACK_WIN;
            long winId = isWhiteWin ? whiteId : blackId;
            long loseId = isWhiteWin ? blackId : whiteId;

            String moveListJson = JSON_MAPPER.writeValueAsString(state.getMoveList());
            EloChangeSet changeSet = userDao.updateStats(winId, loseId);

            dictionaryDao.incrLeaderboardUser(
                    new DictionaryDao.EloChangeSet(winId, changeSet.winEloDiff()),
                    new DictionaryDao.EloChangeSet(loseId, changeSet.loseEloDiff()));
            replayDao.insert(whiteId, blackId, result, cause, changeSet.winEloDiff(), changeSet.loseEloDiff(), moveListJson);
        } catch (Exception ex) {
            LOG.info("Failed to persist game results to database in background thread {}", String.valueOf(ex));
        }
    }

    public ChessRoom forfeit(String gameId, PlayerEntity player) {
        ChessRoom state = dictionaryDao.getGame(gameId);
        if (state == null) {
            return null;
        }

        boolean didBlackForfeit = state.getBlackPlayer().equals(player);

        state.setEnded(true);
        onFinishGame(state, didBlackForfeit, ReplayEntity.FORFEIT);

        return dictionaryDao.setGame(gameId, state); // did black forfeit? then white won.
    }

    public List<ChessRoom> getGames(int page) {
        return dictionaryDao.getGames(page, 20);
    }
}
