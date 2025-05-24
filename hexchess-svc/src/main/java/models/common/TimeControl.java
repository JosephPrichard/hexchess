package models.common;

import static utils.Globals.LOG;

public enum TimeControl {
    REAL_TIME,
    CORRESPONDENCE,
    UNLIMITED;

    public static TimeControl fromString(String value) {
        return switch (value.toUpperCase()) {
            case "REAL_TIME" -> REAL_TIME;
            case "CORRESPONDENCE" -> CORRESPONDENCE;
            case "UNLIMITED" -> UNLIMITED;
            default -> {
                LOG.warn("Unknown time control {}, defaulting to {}", value, UNLIMITED);
                yield UNLIMITED;
            }
        };
    }
}
