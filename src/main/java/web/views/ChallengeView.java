package web.views;

import lombok.Data;
import models.ChallengeEntity;

import java.sql.Timestamp;
import java.time.Duration;
import java.time.format.DateTimeFormatter;

import static daos.ChallengeDao.THRESHOLD_EXPIRATION;

@Data
public class ChallengeView {
    private static final DateTimeFormatter DATE_FORMATTER = DateTimeFormatter.ofPattern("MM/dd/yyyy HH:mm");

    public long challengerId;
    public String challengerName;
    public String challengerCountry;
    public float challengerElo;
    public long challengeeId;
    public String challengeeName;
    public String challengeeCountry;
    public float challengeeElo;
    public String madeAgo;
    public String expiresIn;

    public static ChallengeView fromEntity(ChallengeEntity entity) {
        ChallengeView view = new ChallengeView();
        view.challengerId = entity.challengerId;
        view.challengerName = entity.challengerName;
        view.challengerCountry = entity.challengerCountry;
        view.challengerElo = Math.round(entity.challengerElo);
        view.challengeeId = entity.challengeeId;
        view.challengeeName = entity.challengeeName;
        view.challengeeCountry = entity.challengeeCountry;
        view.challengeeElo = Math.round(entity.challengeeElo);
        view.madeAgo = formatMadeAgo(entity.madeOn);
        view.expiresIn = formatExpiresIn(entity.madeOn);
        return view;
    }

    public static String formatMadeAgo(Timestamp madeOn) {
        long now = System.currentTimeMillis();
        long then = madeOn.getTime();
        long days = Duration.ofMillis(now - then).toDays();
        return days > 0 ? String.format("%s days ago", days) : "Today";
    }

    public static String formatExpiresIn(Timestamp madeOn) {
        long now = System.currentTimeMillis();
        long then = madeOn.getTime();
        long duration = THRESHOLD_EXPIRATION.toMillis();
        long days = Duration.ofMillis(duration - (now - then)).toDays();
        return days > 0 ? String.format("in %s days", days) : "Today";
    }
}
