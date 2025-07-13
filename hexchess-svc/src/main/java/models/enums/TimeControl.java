package models.enums;

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

    public int toInt() {
        return switch (this) {
            case REAL_TIME -> 0;
            case CORRESPONDENCE -> 1;
            case UNLIMITED -> 2;
        };
    }

    public static TimeControl fromInt(int value) {
        return switch (value) {
            case 0 -> REAL_TIME;
            case 1 -> CORRESPONDENCE;
            case 2 -> UNLIMITED;
            default -> {
                LOG.warn("Unknown time control {}, defaulting to {}", value, UNLIMITED);
                yield UNLIMITED;
            }
        };
    }
}
