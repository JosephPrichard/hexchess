package chess;

import com.fasterxml.jackson.databind.annotation.JsonSerialize;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.util.List;
import java.util.function.Function;


@Data
@NoArgsConstructor(force = true)
@AllArgsConstructor
@JsonSerialize(using = BoardSerializer.class)
public class ChessBoard {

    public final static byte EMPTY = 0;
    public final static byte WHITE_PAWN = 1;
    public final static byte BLACK_PAWN = 2;
    public final static byte WHITE_KNIGHT = 3;
    public final static byte BLACK_KNIGHT = 4;
    public final static byte WHITE_BISHOP = 5;
    public final static byte BLACK_BISHOP = 6;
    public final static byte WHITE_ROOK = 7;
    public final static byte BLACK_ROOK = 8;
    public final static byte WHITE_QUEEN = 9;
    public final static byte BLACK_QUEEN = 10;
    public final static byte WHITE_KING = 11;
    public final static byte BLACK_KING = 12;

    public final static int MAX_RANKS = 11; // marked by numbers 1-11
    public final static int FILES = 11; // marked by letters A-L
    public final static int[] RANKS_PER_FILE = {6, 7, 8, 9, 10, 11, 10, 9, 8, 7, 6};

    private Turn turn;
    private final byte[][] pieces; // jagged array storing the pieces with [file][rank] format

    public ChessBoard(Turn turn) {
        pieces = new byte[FILES][];
        for (int i = 0; i < pieces.length; i++) {
            pieces[i] = new byte[RANKS_PER_FILE[i]];
        }
        this.turn = turn;
    }

    public static ChessBoard initial() {
        ChessBoard board = new ChessBoard(Turn.WHITE);

        board.setPiece("b1", WHITE_PAWN);
        board.setPiece("c2", WHITE_PAWN);
        board.setPiece("d3", WHITE_PAWN);
        board.setPiece("e4", WHITE_PAWN);
        board.setPiece("f5", WHITE_PAWN);
        board.setPiece("g4", WHITE_PAWN);
        board.setPiece("h3", WHITE_PAWN);
        board.setPiece("i2", WHITE_PAWN);
        board.setPiece("j1", WHITE_PAWN);

        board.setPiece("c1", WHITE_ROOK);
        board.setPiece("d1", WHITE_KNIGHT);
        board.setPiece("e1", WHITE_QUEEN);
        board.setPiece("f1", WHITE_BISHOP);
        board.setPiece("f2", WHITE_BISHOP);
        board.setPiece("f3", WHITE_BISHOP);
        board.setPiece("g1", WHITE_KING);
        board.setPiece("h1", WHITE_KNIGHT);
        board.setPiece("i1", WHITE_ROOK);

        board.setPiece("b7", BLACK_PAWN);
        board.setPiece("c7", BLACK_PAWN);
        board.setPiece("d7", BLACK_PAWN);
        board.setPiece("e7", BLACK_PAWN);
        board.setPiece("f7", BLACK_PAWN);
        board.setPiece("g7", BLACK_PAWN);
        board.setPiece("h7", BLACK_PAWN);
        board.setPiece("i7", BLACK_PAWN);
        board.setPiece("j7", BLACK_PAWN);

        board.setPiece("c8", BLACK_ROOK);
        board.setPiece("d9", BLACK_KNIGHT);
        board.setPiece("e10", BLACK_QUEEN);
        board.setPiece("f11", BLACK_BISHOP);
        board.setPiece("f10", BLACK_BISHOP);
        board.setPiece("f9", BLACK_BISHOP);
        board.setPiece("g10", BLACK_KING);
        board.setPiece("h9", BLACK_KNIGHT);
        board.setPiece("i8", BLACK_ROOK);

        return board;
    }

    public Turn turn() {
        return turn;
    }

    public void flipTurn() {
        turn = turn.opposite();
    }

    public void setPiece(Hexagon hex, byte piece) {
        setPiece(hex.getFile(), hex.getRank(), piece);
    }

    public byte getPiece(Hexagon hex) {
        return getPiece(hex.getFile(), hex.getRank());
    }

    public void setPiece(int file, int rank, byte piece) {
        assert pieces != null;
        pieces[file][rank] = piece;
    }

    public byte getPiece(int file, int rank) {
        assert pieces != null;
        return pieces[file][rank];
    }

    public void setPiece(String notation, byte piece) {
        setPiece(Hexagon.fromNotation(notation), piece);
    }

    public byte getPiece(String notation) {
        return getPiece(Hexagon.fromNotation(notation));
    }

    public static boolean hasPawnMoved(Hexagon pawnHex, byte piece) {
        // pawns have moved if they exit the rank threshold, based on what color the pawn is
        if (isWhite(piece)) {
            int minRank = switch (pawnHex.getFile()) {
                case 1, 9 -> 0;
                case 2, 8 -> 1;
                case 3, 7 -> 2;
                case 4, 6 -> 3;
                case 5 -> 4;
                default -> -1; // if a pawn is on any other file it must have moved
            };
            return pawnHex.getRank() > minRank;
        } else {
            return pawnHex.getRank() < 7;
        }
    }

    public Hexagon findKing(Turn turn) {
        for (Hexagon hex : Hexagon.ORDERED) {
            if (getPiece(hex) == WHITE_KING && turn.isWhite() || getPiece(hex) == BLACK_KING && turn.isBlack()) {
                return hex;
            }
        }
        throw new IllegalStateException("Board doesn't have a king");
    }

    public static boolean isPieceTurn(byte piece, Turn turn) {
        return piece % 2 == 0 && turn.isBlack() || piece % 2 == 1 && turn.isWhite();
    }

    public static boolean isWhite(byte piece) {
        return piece % 2 == 1;
    }

    public static boolean areOpposite(byte piece1, byte piece2) {
        assert piece1 != EMPTY;
        assert piece2 != EMPTY;
        return piece1 % 2 != piece2 % 2;
    }

    public boolean inBounds(int file, int rank) {
        if (file < 0 || file >= FILES || rank < 0) return false;
        assert pieces != null;
        return rank < pieces[file].length;
    }

    public boolean inBounds(Hexagon hex) {
        return inBounds(hex.getFile(), hex.getRank());
    }

    public static boolean isPawn(byte piece) {
        return piece == WHITE_PAWN || piece == BLACK_PAWN;
    }

    public static boolean isKing(byte piece) {
        return piece == WHITE_KING || piece == BLACK_KING;
    }

    public static char pieceToChar(byte piece) {
        return switch (piece) {
            case EMPTY -> '.';
            case WHITE_PAWN -> 'P';
            case BLACK_PAWN -> 'p';
            case WHITE_KNIGHT -> 'N';
            case BLACK_KNIGHT -> 'n';
            case WHITE_BISHOP -> 'B';
            case BLACK_BISHOP -> 'b';
            case WHITE_ROOK -> 'R';
            case BLACK_ROOK -> 'r';
            case WHITE_QUEEN -> 'Q';
            case BLACK_QUEEN -> 'q';
            case WHITE_KING -> 'K';
            case BLACK_KING -> 'k';
            default -> throw new IllegalStateException("Invalid piece: " + piece);
        };
    }

    @Override
    public String toString() {
        return toString((x) -> false);
    }

    public String toString(Function<Hexagon, Boolean> isMove) {
        StringBuilder sb = new StringBuilder("\n");

        for (int file = 0; file < ChessBoard.FILES; file++) {
            int ranksCount = RANKS_PER_FILE[file];
            int ranksDiff = MAX_RANKS - ranksCount;

            char charStr = (char) (file + 'a');
            sb.append(charStr).append("   ");

            sb.append("  ".repeat(Math.max(0, ranksDiff)));

            for (int rank = 0; rank < ranksCount; rank++) {
                if (isMove.apply(Hexagon.of(file, rank))) {
                    sb.append('x').append("   ");
                } else {
                    byte piece = getPiece(file, rank);
                    sb.append(pieceToChar(piece)).append("   ");
                }
            }

            sb.append("\n");
        }

        return sb.toString();
    }

    public String toMovesString(List<Hexagon> moves) {
        return toString((hex) -> moves.stream().anyMatch((m) -> m.equals(hex)));
    }

    public String toPieceMovesString(List<PieceMoves> moves) {
        boolean[][] isAttacked = new boolean[FILES][];
        for (int i = 0; i < isAttacked.length; i++) {
            isAttacked[i] = new boolean[RANKS_PER_FILE[i]]; // defaulted to false
        }

        for (PieceMoves pm : moves) {
            for (Hexagon move : pm.getMoves()) {
                isAttacked[move.getFile()][move.getRank()] = true;
            }
        }

        return toString((hex) -> isAttacked[hex.getFile()][hex.getRank()]);
    }
}
