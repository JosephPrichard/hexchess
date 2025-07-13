package models.enums;

import models.entities.ReplayEntity;

public enum ReplayResult {
    WHITE_WIN,
    BLACK_WIN,
    DRAW;

    public static ReplayResult fromInteger(int result) {
        return switch (result) {
            case ReplayEntity.DRAW -> DRAW;
            case ReplayEntity.WHITE_WIN -> WHITE_WIN;
            case ReplayEntity.BLACK_WIN -> BLACK_WIN;
            default -> throw new IllegalStateException("Invalid replay result: " + result);
        };
    }
}
