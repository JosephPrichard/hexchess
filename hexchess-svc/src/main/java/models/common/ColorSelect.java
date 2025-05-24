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
}
