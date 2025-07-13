package models.entities;

import lombok.*;

import java.sql.Timestamp;

@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class ChallengeEntity {
    private long challengerId;
    private String challengerName = "";
    private String challengerCountry = "";
    private float challengerElo;
    private long challengeeId;
    private String challengeeName = "";
    private String challengeeCountry = "";
    private float challengeeElo;
    private String timeControl = "";
    private String startColor = ""; // from challenger's perspective.
    @EqualsAndHashCode.Exclude
    private Timestamp madeOn;
}
