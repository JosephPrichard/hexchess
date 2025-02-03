package models;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;

import java.sql.Timestamp;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class Challenge {
    public static final int PENDING = 0;
    public static final int ACCEPTED = 1;
    public static final int REJECTED = 2;

    public String challengeeId;
    public String challengeeUsername;
    public String challengeeCountry;
    public float challengeeElo;
    public String challengerId;
    public String challengerUsername;
    public String challengerCountry;
    public float challengerElo;
    public int status;
    @EqualsAndHashCode.Exclude
    public Timestamp madeOn;

    public boolean isPending() {
        return status == PENDING;
    }

    public boolean isAccepted() {
        return status == ACCEPTED;
    }

    public boolean isRejected() {
        return status == REJECTED;
    }
}
