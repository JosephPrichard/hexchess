package chess;

import lombok.*;
import messages.Messages;

import java.util.ArrayList;
import java.util.List;

@Data
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

    public Messages.PieceMove serialize() {
        return Messages.PieceMove.newBuilder()
            .setPiece(piece)
            .setFrom(from.serialize())
            .setTo(to.serialize())
            .build();
    }

    public static PieceMove deserialize(Messages.PieceMove msg) {
        return new PieceMove((byte) msg.getPiece(), Hexagon.deserialize(msg.getFrom()), Hexagon.deserialize(msg.getTo()));
    }
}
