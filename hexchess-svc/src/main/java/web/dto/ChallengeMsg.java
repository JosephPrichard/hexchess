package web.dto;

import lombok.AllArgsConstructor;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;
import lombok.ToString;
import models.entities.ChallengeEntity;

@EqualsAndHashCode
@ToString
@NoArgsConstructor
@AllArgsConstructor
public class ChallengeMsg {
    public long challengerId;
    public String challengerName;
    public String challengerCountry;
    public long challengeeId;
    public String challengeeName;
    public String challengeeCountry;

    public static ChallengeMsg fromEntity(ChallengeEntity entity) {
        ChallengeMsg msg = new ChallengeMsg();
        msg.challengerId = entity.challengerId;
        msg.challengerName = entity.challengerName;
        msg.challengerCountry = entity.challengerCountry;
        msg.challengeeId = entity.challengeeId;
        msg.challengeeName = entity.challengeeName;
        msg.challengeeCountry = entity.challengeeCountry;
        return msg;
    }
}
