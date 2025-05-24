package chess;

import lombok.*;

import java.util.ArrayList;
import java.util.List;

import static chess.ChessBoard.FILES;
import static chess.ChessBoard.RANKS_PER_FILE;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class Hexagon {
    public static final Hexagon[] ORDERED = getOrdered();
    public static final int MIDPOINT = 5;

    private int file;
    private int rank;

    public static Hexagon of(int file, int rank) {
        return new Hexagon(file, rank);
    }

    public Hexagon walk(Direction[] directions) {
        int file = this.file;
        int rank = this.rank;
        for (Direction direction : directions) {
            switch (direction) {
                case UP -> rank += 1;
                case DOWN -> rank -= 1;
                case UP_LEFT -> {
                    rank += file <= MIDPOINT ? 0 : 1;
                    file -= 1;
                }
                case DOWN_LEFT -> {
                    rank += file <= MIDPOINT ? -1 : 0;
                    file -= 1;
                }
                case UP_RIGHT -> {
                    rank += file < MIDPOINT ? 1 : 0;
                    file += 1;
                }
                case DOWN_RIGHT -> {
                    rank += file < MIDPOINT ? 0 : -1;
                    file += 1;
                }
            }
        }
        return Hexagon.of(file, rank);
    }

    public boolean canPromote() {
        return switch (file) {
            case 0, 10 -> rank >= 5;
            case 1, 9 -> rank >= 6;
            case 2, 8 -> rank >= 7;
            case 3, 7 -> rank >= 8;
            case 4, 6 -> rank >= 9;
            case 5 -> rank >= 10;
            default -> throw new IllegalStateException("Cannot promote to an invalid file " + file);
        };
    }

    private static Hexagon[] getOrdered() {
        List<Hexagon> hexagons = new ArrayList<>();
        for (int file = 0; file < FILES; file++) {
            for (int rank = 0; rank < RANKS_PER_FILE[file]; rank++) {
                hexagons.add(new Hexagon(file, rank));
            }
        }
        return hexagons.toArray(Hexagon[]::new);
    }

    public static Hexagon fromNotation(String notation) {
        int file = notation.charAt(0) - 'a';
        int rank = Integer.parseInt(notation.substring(1)) - 1;
        return new Hexagon(file, rank);
    }

    @Override
    public String toString() {
        char fileChar = (char) (file + 'a');
        return fileChar + Integer.toString(rank + 1);
    }
}
