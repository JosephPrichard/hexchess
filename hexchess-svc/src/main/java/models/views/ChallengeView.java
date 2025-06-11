package models.views;

import lombok.*;
import models.entities.ChallengeEntity;
import models.common.TimeControl;

import java.sql.Timestamp;
import java.time.Duration;

import static services.daos.ChallengeDao.THRESHOLD_EXPIRATION;

@Data
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
        ChallengeView view = new ChallengeView();
        view.challengerId = entity.getChallengerId();
        view.challengerName = entity.getChallengerName();
        view.challengerCountry = entity.getChallengerCountry();
        view.challengerElo = Math.round(entity.getChallengerElo());
        view.challengeeId = entity.getChallengeeId();
        view.challengeeName = entity.getChallengeeName();
        view.challengeeCountry = entity.getChallengeeCountry();
        view.challengeeElo = Math.round(entity.getChallengeeElo());
        view.timeControl = TimeControl.fromString(entity.getTimeControl());
        view.madeAgo = formatMadeAgo(entity.getMadeOn());
        view.expiresIn = formatExpiresIn(entity.getMadeOn());
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