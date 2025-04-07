package chess;

public enum Turn {
    BLACK,
    WHITE;

    public Turn opposite() {
        return this == WHITE ? BLACK : WHITE;
    }

    public boolean isWhite() {
        return this == WHITE;
    }

    public boolean isBlack() {
        return this == BLACK;
    }

    public int toInt() {
        return switch (this) {
            case WHITE -> 0;
            case BLACK -> 1;
        };
    }
}