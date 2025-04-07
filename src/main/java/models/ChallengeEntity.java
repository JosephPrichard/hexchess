package models;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;

import java.sql.Timestamp;
import java.time.Duration;

import static daos.ChallengeDao.THRESHOLD_EXPIRATION;

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

    public int getChallengeeEloFmt() {
        return Math.round(challengeeElo);
    }

    public int getChallengerEloFmt() {
        return Math.round(challengerElo);
    }

    public String getMadeAgo() {
        long now = System.currentTimeMillis();
        long then = madeOn.getTime();
        long days = Duration.ofMillis(now - then).toDays();
        return days > 0 ? String.format("%s days ago", days) : "Today";
    }

    public String getExpiresIn() {
        long now = System.currentTimeMillis();
        long then = madeOn.getTime();
        long duration = THRESHOLD_EXPIRATION.toMillis();
        long days = Duration.ofMillis(duration - (now - then)).toDays();
        return days > 0 ? String.format("in %s days", days) : "Today";
    }
}
