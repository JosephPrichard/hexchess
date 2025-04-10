package models;

import com.fasterxml.jackson.annotation.JsonIgnore;
import chess.*;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;

import java.util.ArrayList;
import java.util.List;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class GameState {
    public String id;
    public ChessGame game;
    public PlayerEntity whitePlayer = null;
    public PlayerEntity blackPlayer = null;
    public boolean isEnded;
    @JsonIgnore
    public Boolean isFirstPlayerWhite = null; // true - first player joining should be white... false - first player joining should be black... null - random...
    @EqualsAndHashCode.Exclude
    @JsonIgnore
    public double touch;
    @JsonIgnore
    public List<PieceMove> moveList;

    public static GameState startWithGame(String id) {
        ChessGame game = ChessGame.start();
        List<PieceMove> moveList = new ArrayList<>();
        return new GameState(id, game, null, null, false, null, 0, moveList);
    }

    public static GameState ofPlayers(String id, PlayerEntity whitePlayer, PlayerEntity blackPlayer) {
        return new GameState(id, null, whitePlayer, blackPlayer, false, null, 0, null);
    }

    public PlayerEntity getCurrPlayer() {
        return game.getBoard().turn().isWhite() ? whitePlayer : blackPlayer;
    }

    public boolean isPlayerTurn(PlayerEntity player) {
        PlayerEntity currPlayer = getCurrPlayer();
        if (currPlayer == null) {
            return false;
        }
        return currPlayer.equals(player);
    }

    public void pushMoveList(PieceMove move) {
        moveList.add(move);
    }

    public static List<PieceMove> randomMoveList() {
        ChessGame game = ChessGame.start();
        List<PieceMove> moveList = new ArrayList<>();

        for (int i = 0; i < 34; i++) {
            game.initPieceMoves();

            List<PieceMoves> currMoves = game.getCurrMoves();

            // we're going to assume there is at least one piece
            PieceMoves pm = currMoves.stream().filter((x) -> !x.getMoves().isEmpty()).findFirst().orElseThrow();
            Hexagon from = pm.getHex();
            Hexagon to = pm.getMoves().getFirst(); // make the first move (we already know there is at least one)

            PieceMove move = new PieceMove(game.getBoard().getPiece(from), from, to);
            game.makeMove(from, to);
            moveList.add(move);
        }
        return moveList;
    }
}
