package chess;

import lombok.*;

import java.util.ArrayList;
import java.util.List;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class PieceMove {
    private byte piece;
    private Hexagon from;
    private Hexagon to;

    public static List<PieceMove> randomMoveList() {
        ChessGame game = ChessGame.start();
        List<PieceMove> moveList = new ArrayList<>();

        for (int i = 0; i < 34; i++) {
            game.initPieceMoves();

            List<PieceMoves> currMoves = game.findCurrMoves();

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
