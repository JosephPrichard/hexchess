package models.views;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.ToString;
import models.entities.ChallengeEntity;
import models.common.TimeControl;

import java.sql.Timestamp;
import java.time.Duration;

import static services.daos.ChallengeDao.THRESHOLD_EXPIRATION;

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
    public String timeControl;
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
        view.timeControl = formatTimeControl(TimeControl.fromString(entity.timeControl));
        view.madeAgo = formatMadeAgo(entity.madeOn);
        view.expiresIn = formatExpiresIn(entity.madeOn);
        return view;
    }

    public static String formatTimeControl(TimeControl mode) {
        return switch (mode) {
            case UNLIMITED -> "Unlimited &#8734;+0";
            case CORRESPONDENCE -> String.format("Correspondence %s+%s", 10, 1);
            case REAL_TIME -> String.format("Realtime %s+%s", 5, 3);
        };
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