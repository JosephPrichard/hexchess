
package models.views;

import lombok.*;
import models.entities.UserEntity;
import utils.Strings;

@Data
@NoArgsConstructor
@AllArgsConstructor
@Builder
public class UserView {
    private long id;
    private String username = "";
    private String country = "";
    private int elo;
    private int highestElo;
    private int wins;
    private int losses;
    private int rank;
    private String bio = "";
    private String joinedOn = "";
    private int total;
    private int winRate;

    public static UserView create(UserEntity entity) {
        int total = entity.getWins() + entity.getLosses();
        int winRate = total == 0 ? 0 : entity.getWins() * 100 / total;

        return UserView.builder()
            .id(entity.getId())
            .username(Strings.sanitize(entity.getUsername()))
            .country(entity.getCountry())
            .elo(Math.round(entity.getElo()))
            .highestElo(Math.round(entity.getHighestElo()))
            .bio(Strings.sanitize(entity.getBio()))
            .joinedOn(Strings.formatTimestamp(entity.getJoinedOn()))
            .wins(entity.getWins())
            .losses(entity.getLosses())
            .total(total)
            .winRate(winRate)
            .rank(entity.getRank())
            .build();
    }
}
