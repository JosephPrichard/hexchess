
package models.views;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.ToString;
import org.jsoup.Jsoup;
import models.entities.UserEntity;
import web.WebConstants;

import java.time.format.DateTimeFormatter;

import static utils.Globals.HTML_SAFELIST;

@ToString
@EqualsAndHashCode
@Getter
public class UserView {
    private static final DateTimeFormatter DATE_FORMATTER = DateTimeFormatter.ofPattern("MM/dd/yyyy");

    public long id;
    public String username;
    public String country;
    public int elo;
    public int highestElo;
    public int wins;
    public int losses;
    public int rank;
    public String bio;
    public String joinedOn;
    public int total;
    public int winRate;

    public static UserView create(UserEntity entity) {
        UserView view = new UserView();
        view.id = entity.id;
        view.username = entity.username != null ? Jsoup.clean(entity.username, HTML_SAFELIST) : null;
        view.country = entity.country;
        view.elo = Math.round(entity.elo);
        view.highestElo = Math.round(entity.highestElo);
        view.bio = entity.bio != null ? Jsoup.clean(entity.bio, HTML_SAFELIST) : null;
        view.joinedOn = entity.joinedOn != null ? entity.joinedOn.toLocalDateTime().format(DATE_FORMATTER) : null;
        view.wins = entity.wins;
        view.losses = entity.losses;
        view.total = entity.wins + entity.losses;
        view.winRate = view.total == 0 ? 0 : entity.wins * 100 / view.total;
        view.rank = entity.rank;
        return view;
    }
}
