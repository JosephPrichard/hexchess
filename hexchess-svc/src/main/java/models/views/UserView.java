
package models.views;

import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.ToString;
import org.jsoup.Jsoup;
import models.entities.UserEntity;
import web.WebConstants;

import java.time.format.DateTimeFormatter;

import static utils.Globals.HTML_SAFELIST;

@Data
public class UserView {
    private static final DateTimeFormatter DATE_FORMATTER = DateTimeFormatter.ofPattern("MM/dd/yyyy");

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
        UserView view = new UserView();
        view.id = entity.getId();
        view.username = entity.getUsername() != null ? Jsoup.clean(entity.getUsername(), HTML_SAFELIST) : null;
        view.country = entity.getCountry();
        view.elo = Math.round(entity.getElo());
        view.highestElo = Math.round(entity.getHighestElo());
        view.bio = entity.getBio() != null ? Jsoup.clean(entity.getBio(), HTML_SAFELIST) : null;
        view.joinedOn = entity.getJoinedOn() != null ? entity.getJoinedOn().toLocalDateTime().format(DATE_FORMATTER) : null;
        view.wins = entity.getWins();
        view.losses = entity.getLosses();
        view.total = view.wins + view.losses;
        view.winRate = view.total == 0 ? 0 : view.wins * 100 / view.total;
        view.rank = entity.getRank();
        return view;
    }
}
