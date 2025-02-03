package models;

import lombok.AllArgsConstructor;
import lombok.Data;

@Data
@AllArgsConstructor
public class ChallengeEntity {
    public static final int PENDING = 0;
    public static final int ACCEPTED = 1;
    public static final int REJECTED = 2;

    public long challengerId;
    public String challengerUsername;
    public String challengerCountry;
    public float challengerElo;
    public long challengedId;
    public String challengeeUsername;
    public String challengeeCountry;
    public float challengeeElo;
    public int status;

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
