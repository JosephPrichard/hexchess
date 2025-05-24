
package models.views;

import chess.PieceMove;
import com.fasterxml.jackson.core.type.TypeReference;
import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.ToString;
import models.common.ReplayCause;
import models.common.ReplayResult;
import org.jsoup.Jsoup;
import models.entities.ReplayEntity;

import java.sql.Timestamp;
import java.time.Duration;
import java.time.format.DateTimeFormatter;
import java.util.List;
import java.util.function.Function;

import static models.entities.ReplayEntity.*;
import static utils.Globals.*;

@ToString
@EqualsAndHashCode
@Getter
public class ReplayView {
    private static final DateTimeFormatter DATE_FORMATTER = DateTimeFormatter.ofPattern("MM/dd/yyyy");

    private long id;
    private long whiteId;
    private long blackId;
    private String whiteName;
    private String blackName;
    private String whiteCountry;
    private String blackCountry;
    private float winElo;
    private float loseElo;
    private float whiteElo;
    private float blackElo;
    private String playedOn;
    private ReplayResult result;
    private ReplayCause cause;
    private float whiteEloDiff;
    private float blackEloDiff;

    public static ReplayView createRow(ReplayEntity entity) {
        return create(entity, ReplayView::formatDate);
    }

    public static ReplayView createHeader(ReplayEntity entity) {
        return create(entity, ReplayView::formatDuration);
    }

    public static ReplayView create(ReplayEntity entity, Function<Timestamp, String> formatPlayedOn) {
        ReplayView view = new ReplayView();
        view.id = entity.getId();
        view.whiteId = entity.getWhiteId();
        view.blackId = entity.getBlackId();
        view.whiteName = entity.getWhiteName() != null ? Jsoup.clean(entity.getWhiteName(), HTML_SAFELIST) : null;
        view.blackName = entity.getBlackName() != null ? Jsoup.clean(entity.getBlackName(), HTML_SAFELIST) : null;
        view.whiteCountry = entity.getWhiteCountry() != null ? Jsoup.clean(entity.getWhiteCountry(), HTML_SAFELIST) : null;
        view.blackCountry = entity.getBlackCountry() != null ? Jsoup.clean(entity.getBlackCountry(), HTML_SAFELIST) : null;
        view.winElo = entity.getWinElo();
        view.loseElo = entity.getLoseElo();
        view.whiteElo = entity.getWhiteElo();
        view.blackElo = entity.getBlackElo();
        view.playedOn = entity.getPlayedOn() != null ? formatPlayedOn.apply(entity.getPlayedOn()) : null;
        view.result = ReplayResult.fromInteger(entity.getResult());
        view.cause = ReplayCause.fromInteger(entity.getCause());
        view.calcResultElos(entity);
        return view;
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

    public void calcResultElos(ReplayEntity entity) {
        switch (entity.getResult()) {
        case WHITE_WIN -> {
            whiteEloDiff = entity.getWinElo();
            blackEloDiff = entity.getLoseElo();
        }
        case BLACK_WIN -> {
            whiteEloDiff = entity.getLoseElo();
            blackEloDiff = entity.getWinElo();
        }
        case DRAW -> {
            whiteEloDiff = 0;
            blackEloDiff = 0;
        }
        default -> throw new IllegalStateException("Invalid result state " + result);
        }
    }
}
