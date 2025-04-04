package models;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;

import java.sql.Timestamp;
import java.time.Duration;
import java.time.format.DateTimeFormatter;

import static services.dao.ChallengeDao.THRESHOLD_EXPIRATION;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class ChallengeEntity {
    private static final DateTimeFormatter DATE_FORMATTER = DateTimeFormatter.ofPattern("MM/dd/yyyy HH:mm");

    public long challengerId;
    public String challengerName;
    public float challengerElo;
    public long challengeeId;
    public String challengeeName;
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
