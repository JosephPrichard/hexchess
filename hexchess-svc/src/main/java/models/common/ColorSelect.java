package models.common;

import static utils.Globals.LOG;

public enum ColorSelect {
    WHITE,
    BLACK,
    RANDOM;

    public static ColorSelect fromString(String value) {
        return switch (value.toUpperCase()) {
            case "WHITE" -> WHITE;
            case "BLACK" -> BLACK;
            case "RANDOM" -> RANDOM;
            default -> {
                LOG.warn("Unknown color select {}, defaulting to {}", value, RANDOM);
                yield RANDOM;
            }
        };
    }

    public int toInt() {
        return switch (this) {
            case WHITE -> 0;
            case BLACK -> 1;
            case RANDOM -> 2;
        };
    }

    public static ColorSelect fromInt(int value) {
        return switch (value) {
            case 0 -> WHITE;
            case 1 -> BLACK;
            case 2 -> RANDOM;
            default -> throw new IllegalStateException("Invalid value for color select: " + value);
        };
    }
}
