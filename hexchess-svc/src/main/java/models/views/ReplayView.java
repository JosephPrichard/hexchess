
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
    private static final TypeReference<List<PieceMove>> MOVE_LIST_TYPE = new TypeReference<>() {};
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
    public float whiteElo;
    public float blackElo;
    public List<PieceMove> moveList;
    public String playedOn;
    public ReplayResult result;
    public ReplayCause cause;
    public float whiteEloDiff;
    public float blackEloDiff;
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
        view.whiteElo = entity.whiteElo;
        view.blackElo = entity.blackElo;
        view.moveList = deserializeMoveList(entity.moveListJson);
        view.playedOn = entity.playedOn != null ? formatPlayedOn.apply(entity.playedOn) : null;
        view.result = ReplayResult.fromInteger(entity.result);
        view.cause = ReplayCause.fromInteger(entity.cause);
        view.calcResultElos(entity);
        return view;
    }

    public static List<PieceMove> deserializeMoveList(String moveListJson) {
        try {
            return moveListJson != null ? JSON_MAPPER.readValue(moveListJson, MOVE_LIST_TYPE) : null;
        } catch (Exception e) {
            LOGGER.error("Failed to deserialize moveList={}", moveListJson);
            throw new RuntimeException(e);
        }
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
        switch (entity.result) {
        case WHITE_WIN -> {
            whiteEloDiff = entity.winElo;
            blackEloDiff = entity.loseElo;
        }
        case BLACK_WIN -> {
            whiteEloDiff = entity.loseElo;
            blackEloDiff = entity.winElo;
        }
        case DRAW -> {
            whiteEloDiff = 0;
            blackEloDiff = 0;
        }
        default -> throw new IllegalStateException("Invalid result state " + result);
        }
    }
}
