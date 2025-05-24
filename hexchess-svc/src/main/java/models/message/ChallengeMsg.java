package models.message;

import lombok.*;
import models.entities.ChallengeEntity;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class ChallengeMsg {
    private long challengerId;
    private String challengerName;
    private String challengerCountry;
    private long challengeeId;
    private String challengeeName;
    private String challengeeCountry;

    public static ChallengeMsg fromEntity(ChallengeEntity entity) {
        ChallengeMsg msg = new ChallengeMsg();
        msg.challengerId = entity.getChallengerId();
        msg.challengerName = entity.getChallengerName();
        msg.challengerCountry = entity.getChallengerCountry();
        msg.challengeeId = entity.getChallengeeId();
        msg.challengeeName = entity.getChallengeeName();
        msg.challengeeCountry = entity.getChallengeeCountry();
        return msg;
    }
}
