package web.views;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.ToString;
import models.ChallengeEntity;

import java.sql.Timestamp;
import java.time.Duration;

import static daos.ChallengeDao.THRESHOLD_EXPIRATION;

@ToString
@EqualsAndHashCode
@Getter
public class ChallengeView {
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

    public static ChallengeView create(ChallengeEntity entity) {
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