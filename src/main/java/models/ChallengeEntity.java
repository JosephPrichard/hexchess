package models;

import lombok.*;

import java.sql.Timestamp;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class ChallengeEntity {
    public long challengerId;
    public String challengerName;
    public String challengerCountry;
    public float challengerElo;
    public long challengeeId;
    public String challengeeName;
    public String challengeeCountry;
    public float challengeeElo;
    @EqualsAndHashCode.Exclude
    public Timestamp madeOn;
}
