package models;

import com.fasterxml.jackson.annotation.JsonIgnore;
import com.fasterxml.jackson.core.JsonProcessingException;
import domain.ChessGame;
import domain.Move;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;

import java.util.ArrayList;
import java.util.List;

import static utils.Globals.JSON_MAPPER;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class GameState {
    public String id;
    public ChessGame game;
    public Player whitePlayer = null;
    public Player blackPlayer = null;
    public boolean isEnded;
    @JsonIgnore
    public Boolean isFirstPlayerWhite = null; // true - first player joining should be white... false - first player joining should be black... null - random...
    @EqualsAndHashCode.Exclude
    @JsonIgnore
    public double touch;
    @JsonIgnore
    public List<Move> moveList;

    public GameState deepCopy() {
        return new GameState(id,
            game != null ? game.deepCopy() : null,
            whitePlayer != null ? whitePlayer.deepCopy() : null,
            blackPlayer != null ? blackPlayer.deepCopy() : null,
            isEnded,
            isFirstPlayerWhite,
            touch,
            moveList != null ? moveList.stream().toList() : null);
    }

    public static GameState startWithGame(String id) {
        var game = ChessGame.start();
        List<Move> moveList = new ArrayList<>();
        return new GameState(id, game, null, null, false, null, 0, moveList);
    }

    public static GameState ofPlayers(String id, Player whitePlayer, Player blackPlayer) {
        return new GameState(id, null, whitePlayer, blackPlayer, false, null, 0, null);
    }

    public Player getCurrPlayer() {
        return game.getBoard().turn().isWhite() ? whitePlayer : blackPlayer;
    }

    public boolean isPlayerTurn(Player player) {
        var currPlayer = getCurrPlayer();
        if (currPlayer == null) {
            return false;
        }
        return currPlayer.equals(player);
    }

    public void pushMoveHistory(Move move) {
        moveList.add(move);
    }

    public static String randomAsJson() {
        try {
            var moveList = applyRandomSequence(10);
            return JSON_MAPPER.writeValueAsString(moveList);
        } catch (JsonProcessingException ex) {
            throw new RuntimeException(ex);
        }
    }

    public static List<Move> applyRandomSequence(int count) {
        var game = ChessGame.start();
        List<Move> moveList = new ArrayList<>();

        for (int i = 0; i < count; i++) {
            game.initPieceMoves();

            var currMoves = game.getCurrMoves();

            // we're going to assume there is at least one piece
            var pm = currMoves.stream().filter((x) -> !x.getMoves().isEmpty()).findFirst().orElseThrow();
            var from = pm.getHex();
            var to = pm.getMoves().getFirst(); // make the first move (we already know there is at least one)

            var move = new Move(from, to);
            game.makeMove(move);
            moveList.add(move);
        }
        return moveList;
    }
}
