package chess;

import it.unimi.dsi.fastutil.bytes.ByteArrayList;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.util.ArrayList;
import java.util.List;
import java.util.function.Function;

import static chess.ChessBoard.*;
import static chess.Direction.*;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class ChessGame {

    private ChessBoard board;
    private List<PieceMoves> whiteMoves;
    private List<PieceMoves> blackMoves;
    private ByteArrayList takenWhitePieces = new ByteArrayList();
    private ByteArrayList takenBlackPieces = new ByteArrayList();

    public static ChessGame start() {
        return new ChessGame(ChessBoard.initial());
    }

    public static ChessGame empty() {
        return new ChessGame(new ChessBoard(Turn.WHITE));
    }

    public ChessGame(ChessBoard board) {
        this.board = board;
    }

    public List<PieceMoves> getPieceMoves(Turn turn) {
        return turn.isWhite() ? whiteMoves : blackMoves;
    }

    public List<PieceMoves> getCurrMoves() {
        return getPieceMoves(board.turn());
    }

    public List<PieceMoves> getOppositeMoves() {
        return getPieceMoves(board.turn().opposite());
    }

    public ChessGame setPiece(String notation, byte piece) {
        board.setPiece(notation, piece);
        return this;
    }

    public boolean isValidMove(Move move) {
        assert move != null;
        assert whiteMoves != null;
        assert blackMoves != null;

        List<PieceMoves> moves = getCurrMoves();

        // has a match for a move from one hexagon to another hexagon
        return moves
            .stream()
            .anyMatch((pm) -> {
                boolean isFrom = pm.getHex().equals(move.getFrom());
                boolean hasTo = pm.getMoves().stream().anyMatch((m) -> m.equals(move.getTo()));
                return isFrom && hasTo;
            });
    }

    public void makeMove(Move move) {
        makeMove(move.getFrom(), move.getTo());
    }

    public void makeMove(Hexagon from, Hexagon to) {
        byte piece1 = board.getPiece(from);
        byte piece2 = board.getPiece(to);

        if (piece2 != EMPTY) {
            if (board.turn().isWhite()) {
                takenWhitePieces.add(piece2);
            } else {
                takenBlackPieces.add(piece2);
            }
        }

        board.setPiece(from, EMPTY);
        board.setPiece(to, piece1);
        board.flipTurn();

        whiteMoves = null;
        blackMoves = null;
    }

    public void initPieceMoves() {
        // we find the piece moves for all other pieces besides the king
        whiteMoves = findPieceMoves(Turn.WHITE);
        blackMoves = findPieceMoves(Turn.BLACK);

        Hexagon whiteKingHex = board.findKing(Turn.WHITE);
        Hexagon blackKingHex = board.findKing(Turn.BLACK);

        // find the moves for both kings - excluding any attacking squares
        PieceMoves whiteKingMoves = new PieceMoves(whiteKingHex, findKingMoves(whiteKingHex));
        PieceMoves blackKingMoves = new PieceMoves(blackKingHex, findKingMoves(blackKingHex));
        Hexagon kingHex = board.turn().isWhite() ? whiteKingHex : blackKingHex;

        // decide whether we will add the piece moves... are we in check?
        // we don't need to check if the opposite move is in check... it should never be!
        List<PieceMoves> currMoves = getCurrMoves();
        List<PieceMoves> oppMoves = getOppositeMoves();
        boolean[][] isAttacked = findAttacking(oppMoves);
        boolean isCheck = isAttacked[kingHex.getFile()][kingHex.getRank()];
        if (isCheck) {
            // if the current king is in check, we cannot move any other pieces
            // TODO: add support for maintaining all "blocking" moves
            currMoves.clear();
        }

        // now, we can add the king moves
        whiteMoves.add(whiteKingMoves);
        blackMoves.add(blackKingMoves);
    }

    public boolean[][] findAttacking(List<PieceMoves> moves) {
        boolean[][] isAttacked = new boolean[FILES][];
        for (int i = 0; i < isAttacked.length; i++) {
            isAttacked[i] = new boolean[RANKS_PER_FILE[i]]; // defaulted to false
        }

        for (PieceMoves pm : moves) {
            for (Hexagon move : pm.getMoves()) {
                // make an exception for the pawn... which does not attack when moving ahead
                byte piece = board.getPiece(pm.getHex());
                boolean isMovingAhead = move.getRank() > pm.getHex().getRank();
                if (isPawn(piece) && isMovingAhead) {
                    continue;
                }
                isAttacked[move.getFile()][move.getRank()] = true;
            }
        }
        return isAttacked;
    }

    public boolean isCheckmate() {
        Turn turn = board.turn();
        Hexagon kingHex = board.findKing(turn);
        List<PieceMoves> pieceMoves = getPieceMoves(turn);
        List<PieceMoves> oppPieceMoves = getPieceMoves(turn.opposite());
        PieceMoves kingMoves = pieceMoves.getLast();

        // the LAST element should always be the king moves!
        assert (board.getPiece(kingMoves.getHex()) == (turn.isWhite() ? WHITE_KING : BLACK_KING));

        // a king must be checked to be in checkmate
        boolean[][] isAttacked = findAttacking(oppPieceMoves);
        boolean isChecked = isAttacked[kingHex.getFile()][kingHex.getRank()];
        if (!isChecked) {
            return false;
        }

        // and all hexagons it can move to must be attacked (aka the opponent can move there)
        for (Hexagon move : kingMoves.getMoves()) {
            if (!isAttacked[move.getFile()][move.getRank()])
                return false;
        }

        // TODO: add support for maintaining all "blocking" moves
        // TODO: add support for preventing taking defended attackers
        return true;
    }

    // finds all pieces moves excluding the king moves, which are handled elsewhere
    public List<PieceMoves> findPieceMoves(Turn turn) {
        List<PieceMoves> moves = new ArrayList<>();
        for (Hexagon hex : Hexagon.ORDERED) {
            byte piece = board.getPiece(hex);
            if (piece != EMPTY && isPieceTurn(piece, turn)) {
                // we check the piece type to find the right piece moves (we have already checked the color)
                switch (piece) {
                    case WHITE_ROOK, BLACK_ROOK -> moves.add(findRookMoves(hex));
                    case WHITE_BISHOP, BLACK_BISHOP -> moves.add(findBishopMoves(hex));
                    case WHITE_QUEEN, BLACK_QUEEN -> moves.add(findQueenMoves(hex));
                    case WHITE_KNIGHT, BLACK_KNIGHT -> moves.add(findKnightMoves(hex));
                    case WHITE_PAWN, BLACK_PAWN -> moves.add(findPawnMoves(hex, turn));
                    case WHITE_KING, BLACK_KING -> {
                    } // this is a no-op, we find the king moves in a separate function
                    default -> throw new IllegalStateException("Board has invalid piece " + piece + " at hexagon " + hex);
                }
            }
        }
        return moves;
    }

    public PieceMoves findRookMoves(Hexagon hex) {
        return new PieceMoves(hex, findMovesByTraveling(hex, ROOK_OFFSETS));
    }

    public PieceMoves findBishopMoves(Hexagon hex) {
        return new PieceMoves(hex, findMovesByTraveling(hex, BISHOP_OFFSETS));
    }

    public PieceMoves findQueenMoves(Hexagon hex) {
        return new PieceMoves(hex, findMovesByTraveling(hex, KING_OFFSETS));
    }

    public PieceMoves findKnightMoves(Hexagon hex) {
        return new PieceMoves(hex, findOffsetMoves(hex, KNIGHT_OFFSETS));
    }

    // finds ALL the king moves and returns it directly to a list
    public List<Hexagon> findKingMoves(Hexagon hex) {
        // we want all moves, so nothing is attacking
        return findKingMoves(hex, (x) -> true);
    }

    public List<Hexagon> findKingMoves(Hexagon hex, Function<Hexagon, Boolean> isNotAttacked) {
        return findOffsetMoves(hex, KING_OFFSETS, isNotAttacked);
    }

    private static final Direction[] WHITE_AHEAD = {Direction.UP};
    private static final Direction[] WHITE_TAKE_LEFT = {Direction.UP_LEFT};
    private static final Direction[] WHITE_TAKE_RIGHT = {Direction.UP_RIGHT};
    private static final Direction[] BLACK_AHEAD = {Direction.DOWN};
    private static final Direction[] BLACK_TAKE_LEFT = {Direction.DOWN_LEFT};
    private static final Direction[] BLACK_TAKE_RIGHT = {Direction.DOWN_RIGHT};

    public PieceMoves findPawnMoves(Hexagon hex, Turn turn) {
        byte basePiece = board.getPiece(hex);
        List<Hexagon> moves = new ArrayList<>();

        // we can always move one rank ahead on the same file
        Hexagon move1 = hex.walk(turn.isWhite() ? WHITE_AHEAD : BLACK_AHEAD);
        if (board.inBounds(move1)) {
            byte piece = board.getPiece(move1);
            if (piece == EMPTY) {
                moves.add(move1);
            }
        }

        // we can move a rank ahead of that if we haven't moved yet!
        Hexagon move2 = move1.walk(turn.isWhite() ? WHITE_AHEAD : BLACK_AHEAD);
        if (board.inBounds(move2) && !hasPawnMoved(hex, basePiece)) {
            byte piece = board.getPiece(move2);
            if (piece == EMPTY) {
                moves.add(move2);
            }
        }

        // we can also take in adjacent ranks
        Hexagon move3 = hex.walk(turn.isWhite() ? WHITE_TAKE_LEFT : BLACK_TAKE_LEFT);
        if (board.inBounds(move3)) {
            byte piece = board.getPiece(move3);
            if (piece != EMPTY && areOpposite(basePiece, piece)) {
                moves.add(move3);
            }
        }

        Hexagon move4 = hex.walk(turn.isWhite() ? WHITE_TAKE_RIGHT : BLACK_TAKE_RIGHT);
        if (board.inBounds(move4)) {
            byte piece = board.getPiece(move4);
            if (piece != EMPTY && areOpposite(basePiece, piece)) {
                moves.add(move4);
            }
        }

        return new PieceMoves(hex, moves);
    }

    // travel along the offsets - aka keep on going until we can't move in that direction
    public List<Hexagon> findMovesByTraveling(Hexagon hex, Direction[][] directions) {
        byte basePiece = board.getPiece(hex);
        List<Hexagon> moves = new ArrayList<>();

        for (Direction[] direction : directions) {
            Hexagon move = hex;

            while (true) {
                move = move.walk(direction);

                if (!board.inBounds(move)) {
                    break;
                }

                byte piece = board.getPiece(move);
                if (piece == EMPTY) {
                    // we can move here and keep going!
                    moves.add(move);
                } else {
                    // we cannot keep going - maybe we can take if the piece is opposite color
                    if (areOpposite(piece, basePiece)) {
                        moves.add(move);
                    }
                    break;
                }
            }
        }

        return moves;
    }

    public List<Hexagon> findOffsetMoves(Hexagon hex, Direction[][] directions) {
        return findOffsetMoves(hex, directions, (x) -> true);
    }

    public List<Hexagon> findOffsetMoves(Hexagon hex, Direction[][] directions, Function<Hexagon, Boolean> canMoveTo) {
        byte basePiece = board.getPiece(hex);
        List<Hexagon> moves = new ArrayList<>();

        for (Direction[] direction : directions) {
            Hexagon move = hex.walk(direction);
            if (!board.inBounds(move)) {
                continue;
            }

            byte piece = board.getPiece(move);
            boolean canMoveHex = piece == EMPTY || areOpposite(basePiece, piece); // short-circuiting prevents us from checking if an empty piece is opposite

            if (canMoveHex && canMoveTo.apply(move)) {
                moves.add(move);
            }
        }

        return moves;
    }
}
