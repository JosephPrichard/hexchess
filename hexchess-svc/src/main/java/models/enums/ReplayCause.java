package models.enums;

import models.entities.ReplayEntity;

public enum ReplayCause {
    CHECKMATE,
    FORFEIT;

    public static ReplayCause fromInteger(int result) {
        return switch (result) {
            case ReplayEntity.CHECKMATE -> CHECKMATE;
            case ReplayEntity.FORFEIT -> FORFEIT;
            default -> throw new IllegalStateException("Invalid replay result: " + result);
        };
    }
}
