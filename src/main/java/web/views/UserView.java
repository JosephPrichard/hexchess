package web.views;

import lombok.Data;
import org.jsoup.Jsoup;
import models.UserEntity;
import web.Constants;

import java.time.format.DateTimeFormatter;

import static utils.Globals.HTML_SAFELIST;

@Data
public class UserView {
    private static final DateTimeFormatter DATE_FORMATTER = DateTimeFormatter.ofPattern("MM/dd/yyyy");

    public long id;
    public String username;
    public String country;
    public int eloFmt;
    public float highestElo;
    public int wins;
    public int losses;
    public int rank;
    public String bio;
    public String joinedOn;
    public int total;
    public int winRate;
    public String winRateColor;

    public static UserView fromEntity(UserEntity entity) {
        UserView view = new UserView();
        view.id = entity.id;
        if (entity.username != null) {
            view.username = Jsoup.clean(entity.username, HTML_SAFELIST);
        }
        view.country = entity.country;
        view.eloFmt = Math.round(entity.elo);
        view.highestElo = Math.round(entity.highestElo);
        if (entity.bio != null) {
            view.bio = Jsoup.clean(entity.bio, HTML_SAFELIST);
        }
        if (entity.joinedOn != null) {
            view.joinedOn = entity.joinedOn.toLocalDateTime().format(DATE_FORMATTER);
        }
        view.wins = entity.wins;
        view.losses = entity.losses;
        view.total = entity.wins + entity.losses;
        view.winRate = view.total == 0 ? 0 : entity.wins * 100 / view.total;
        view.winRateColor = getWinrateColor(view.winRate);
        view.rank = entity.rank;
        return view;
    }

    public static String getWinrateColor(int winRate) {
        if (winRate > 50) {
            return Constants.GREEN_COLOR;
        } else if (winRate < 50) {
            return Constants.RED_COLOR;
        } else {
            return Constants.YELLOW_COLOR;
        }
    }
}
