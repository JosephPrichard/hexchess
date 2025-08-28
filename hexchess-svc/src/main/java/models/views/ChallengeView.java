package models.views;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;
import models.entities.ChallengeEntity;
import models.enums.TimeControl;

import java.sql.Timestamp;
import java.time.Duration;

import static services.ChallengeDao.THRESHOLD_EXPIRATION;

@Data
@NoArgsConstructor
@AllArgsConstructor
@Builder
public class ChallengeView {
    private long challengerId;
    private String challengerName = "";
    private String challengerCountry = "";
    private float challengerElo;
    private long challengeeId;
    private String challengeeName = "";
    private String challengeeCountry = "";
    private float challengeeElo;
    private TimeControl timeControl = TimeControl.REAL_TIME;
    private String madeAgo = "";
    private String expiresIn = "";

    public static ChallengeView create(ChallengeEntity entity) {
        return ChallengeView.builder()
            .challengerId(entity.getChallengerId())
            .challengerName(entity.getChallengerName())
            .challengerCountry(entity.getChallengerCountry())
            .challengerElo(Math.round(entity.getChallengerElo()))
            .challengeeId(entity.getChallengeeId())
            .challengeeName(entity.getChallengeeName())
            .challengeeCountry(entity.getChallengeeCountry())
            .challengeeElo(Math.round(entity.getChallengeeElo()))
            .timeControl(TimeControl.fromString(entity.getTimeControl()))
            .madeAgo(formatMadeAgo(entity.getMadeOn()))
            .expiresIn(formatExpiresIn(entity.getMadeOn()))
            .build();
    }

    public static String formatMadeAgo(Timestamp madeOn) {
        if (madeOn == null) {
            return "";
        }
        long now = System.currentTimeMillis();
        long then = madeOn.getTime();
        long days = Duration.ofMillis(now - then).toDays();
        return days > 0 ? String.format("%s days ago", days) : "Today";
    }

    public static String formatExpiresIn(Timestamp madeOn) {
        if (madeOn == null) {
            return "";
        }
        long now = System.currentTimeMillis();
        long then = madeOn.getTime();
        long duration = THRESHOLD_EXPIRATION.toMillis();
        long days = Duration.ofMillis(duration - (now - then)).toDays();
        return days > 0 ? String.format("in %s days", days) : "Today";
    }
}