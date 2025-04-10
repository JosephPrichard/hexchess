package web.views;

import lombok.Data;
import org.jsoup.Jsoup;
import models.UserEntity;
import web.WebConstants;

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

    public static UserView create(UserEntity entity) {
        UserView view = new UserView();
        view.id = entity.id;
        view.username = entity.username != null ? Jsoup.clean(entity.username, HTML_SAFELIST) : null;
        view.country = entity.country;
        view.eloFmt = Math.round(entity.elo);
        view.highestElo = Math.round(entity.highestElo);
        view.bio = entity.bio != null ? Jsoup.clean(entity.bio, HTML_SAFELIST) : null;
        view.joinedOn = entity.joinedOn != null ? entity.joinedOn.toLocalDateTime().format(DATE_FORMATTER) : null;
        view.wins = entity.wins;
        view.losses = entity.losses;
        view.total = entity.wins + entity.losses;
        view.winRate = view.total == 0 ? 0 : entity.wins * 100 / view.total;
        view.winRateColor = formatWinrateColor(view.winRate);
        view.rank = entity.rank;
        return view;
    }

    public static String formatWinrateColor(int winRate) {
        if (winRate > 50) {
            return WebConstants.GREEN_COLOR;
        } else if (winRate < 50) {
            return WebConstants.RED_COLOR;
        } else {
            return WebConstants.YELLOW_COLOR;
        }
    }
}
