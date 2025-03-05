package models;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;

import java.sql.Timestamp;
import java.time.format.DateTimeFormatter;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class Challenge {
    private static final DateTimeFormatter DATE_FORMATTER = DateTimeFormatter.ofPattern("MM/dd/yyyy");
    public static final int PENDING = 0;
    public static final int ACCEPTED = 1;
    public static final int REJECTED = 2;

    public String challengeeId;
    public String challengeeUsername;
    public float challengeeElo;
    public String challengerId;
    public String challengerUsername;
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

    public String getFormattedPlayedOn() {
        return madeOn.toLocalDateTime().format(DATE_FORMATTER);
    }
}
