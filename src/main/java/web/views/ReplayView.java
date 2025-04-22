
package web.views;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.Setter;
import lombok.ToString;
import org.jsoup.Jsoup;
import models.ReplayEntity;
import web.WebConstants;

import java.sql.Timestamp;
import java.time.Duration;
import java.time.format.DateTimeFormatter;
import java.util.function.Function;

import static models.ReplayEntity.*;
import static utils.Globals.HTML_SAFELIST;

@ToString
@EqualsAndHashCode
@Getter
public class ReplayView {
    private static final DateTimeFormatter DATE_FORMATTER = DateTimeFormatter.ofPattern("MM/dd/yyyy");

    public long id;
    public long whiteId;
    public long blackId;
    public String whiteName;
    public String blackName;
    public String whiteCountry;
    public String blackCountry;
    public float winElo;
    public float loseElo;
    public String whiteElo;
    public String blackElo;
    public String moveListJson;
    public String playedOn;
    public String result;
    public String cause;
    public String whiteEloDiff;
    public String blackEloDiff;
    public String whiteEloColor;
    public String blackEloColor;

    public static ReplayView createRow(ReplayEntity entity) {
        return create(entity, ReplayView::formatDate);
    }

    public static ReplayView createHeader(ReplayEntity entity) {
        return create(entity, ReplayView::formatDuration);
    }

    public static ReplayView create(ReplayEntity entity, Function<Timestamp, String> formatPlayedOn) {
        ReplayView view = new ReplayView();
        view.id = entity.id;
        view.whiteId = entity.whiteId;
        view.blackId = entity.blackId;
        view.whiteName = entity.whiteName != null ? Jsoup.clean(entity.whiteName, HTML_SAFELIST) : null;
        view.blackName = entity.blackName != null ? Jsoup.clean(entity.blackName, HTML_SAFELIST) : null;
        view.whiteCountry = entity.whiteCountry != null ? Jsoup.clean(entity.whiteCountry, HTML_SAFELIST) : null;
        view.blackCountry = entity.blackCountry != null ? Jsoup.clean(entity.blackCountry, HTML_SAFELIST) : null;
        view.winElo = entity.winElo;
        view.loseElo = entity.loseElo;
        view.whiteElo = String.format("%.0f", entity.whiteElo);
        view.blackElo = String.format("%.0f", entity.blackElo);
        view.moveListJson = entity.moveListJson;
        view.playedOn = entity.playedOn != null ? formatPlayedOn.apply(entity.playedOn) : null;
        view.cause = formatCause(entity.cause);
        view.formatResults(entity);
        return view;
    }

    public static String formatCause(int cause) {
        return switch (cause) {
            case CHECKMATE -> "Checkmate";
            case FORFEIT -> "Forfeit";
            default -> throw new IllegalStateException("Invalid cause state " + cause);
        };
    }

    public static String formatDuration(Timestamp timestamp) {
        long now = System.currentTimeMillis();
        long then = timestamp.getTime();
        Duration duration = Duration.ofMillis(now - then);
        if (duration.toDays() > 0) {
            return formatDate(timestamp);
        } else if (duration.toMinutes() >= 60) {
            return String.format("%s hours ago", duration.toHours());
        } else if (duration.toHours() == 1) {
            return "1 hour ago";
        } else if (duration.toMinutes() == 1) {
            return "1 minute ago";
        } else {
            return String.format("%s minutes ago", duration.toMinutes());
        }
    }

    public static String formatDate(Timestamp timestamp) {
        return timestamp.toLocalDateTime().format(DATE_FORMATTER);
    }

    public void formatResults(ReplayEntity entity) {
        switch (entity.result) {
        case WHITE_WIN -> {
            result = "White Victory";
            whiteEloDiff = formatElo(entity.winElo);
            blackEloDiff = formatElo(entity.loseElo);
            whiteEloColor = WebConstants.RED_COLOR;
            blackEloColor = WebConstants.GREEN_COLOR;
        }
        case BLACK_WIN -> {
            result = "Black Victory";
            whiteEloDiff = formatElo(entity.loseElo);
            blackEloDiff = formatElo(entity.winElo);
            whiteEloColor = WebConstants.GREEN_COLOR;
            blackEloColor = WebConstants.RED_COLOR;
        }
        case DRAW -> {
            result = "Draw";
            whiteEloDiff = formatElo(0);
            blackEloDiff = formatElo(0);
            whiteEloColor = WebConstants.YELLOW_COLOR;
            blackEloColor = WebConstants.YELLOW_COLOR;
        }
        default -> throw new IllegalStateException("Invalid result state " + result);
        }
    }

    private static String formatElo(float elo) {
        return (elo >= 0 ? "+" : "") + elo;
    }
}
