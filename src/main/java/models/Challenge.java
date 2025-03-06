package models;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;
import utils.Globals;

import java.sql.Timestamp;
import java.time.format.DateTimeFormatter;

import static dao.ChallengeDao.THRESHOLD_EXPIRATION;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class Challenge {
    private static final DateTimeFormatter DATE_FORMATTER = DateTimeFormatter.ofPattern("MM/dd/yyyy HH:mm");
    public static final int PENDING = 0;
    public static final int ACCEPTED = 1;
    public static final int REJECTED = 2;

    public String challengeeId;
    public String challengeeName;
    public float challengeeElo;
    public String challengerId;
    public String challengerName;
    public float challengerElo;
    public int status;
    @EqualsAndHashCode.Exclude
    public Timestamp madeOn;

    public String getFormattedStatus() {
        return switch (status) {
            case PENDING -> "Pending";
            case ACCEPTED -> "Accepted";
            case REJECTED -> "Rejected";
            default -> throw new IllegalStateException("Invalid status state " + status);
        };
    }

    public String getStatusColor() {
        return switch (status) {
            case PENDING -> Globals.YELLOW_COLOR;
            case ACCEPTED -> Globals.GREEN_COLOR;
            case REJECTED -> Globals.RED_COLOR;
            default -> throw new IllegalStateException("Invalid status state " + status);
        };
    }

    public int getRoundedChallengeeElo() {
        return Math.round(challengeeElo);
    }

    public int getRoundedChallengerElo() {
        return Math.round(challengerElo);
    }

    public String getFormattedMadeOn() {
        return madeOn.toLocalDateTime().format(DATE_FORMATTER);
    }

    public String getFormattedExpiresOn() {
        return madeOn.toLocalDateTime().plusDays(THRESHOLD_EXPIRATION.toDays()).format(DATE_FORMATTER);
    }
}
