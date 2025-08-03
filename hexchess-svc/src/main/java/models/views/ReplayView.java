
package models.views;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;
import models.entities.ReplayEntity;
import models.enums.ReplayCause;
import models.enums.ReplayResult;
import utils.Strings;

import java.sql.Timestamp;
import java.time.Duration;
import java.util.function.Function;

import static models.entities.ReplayEntity.*;

@Data
@NoArgsConstructor
@AllArgsConstructor
@Builder
public class ReplayView {
    private long id;
    private long whiteId;
    private long blackId;
    private String whiteName = "";
    private String blackName = "";
    private String whiteCountry = "";
    private String blackCountry = "";
    private float winElo;
    private float loseElo;
    private float whiteElo;
    private float blackElo;
    private String playedOn = "";
    private ReplayResult result = ReplayResult.DRAW;
    private ReplayCause cause = ReplayCause.CHECKMATE;
    private float whiteEloDiff;
    private float blackEloDiff;

    public static ReplayView createRow(ReplayEntity entity) {
        return create(entity, Strings::formatTimestamp);
    }

    public static ReplayView createHeader(ReplayEntity entity) {
        return create(entity, ReplayView::formatDuration);
    }

    public static ReplayView create(ReplayEntity entity, Function<Timestamp, String> formatPlayedOn) {
        ColorElos results = createResultElos(entity.getResult(), entity.getWinElo(), entity.getLoseElo());

        return ReplayView.builder()
            .id(entity.getId())
            .whiteId(entity.getWhiteId())
            .blackId(entity.getBlackId())
            .whiteName(Strings.sanitize(entity.getWhiteName()))
            .blackName(Strings.sanitize(entity.getBlackName()))
            .whiteCountry(Strings.sanitize(entity.getWhiteCountry()))
            .blackCountry(Strings.sanitize(entity.getBlackCountry()))
            .winElo(entity.getWinElo())
            .loseElo(entity.getLoseElo())
            .whiteElo(entity.getWhiteElo())
            .blackElo(entity.getBlackElo())
            .playedOn(formatPlayedOn.apply(entity.getPlayedOn()))
            .result(ReplayResult.fromInteger(entity.getResult()))
            .cause(ReplayCause.fromInteger(entity.getCause()))
            .whiteEloDiff(results.whiteEloDiff())
            .blackEloDiff(results.blackEloDiff())
            .build();
    }

    public static String formatDuration(Timestamp timestamp) {
        if (timestamp == null) {
            return "";
        }

        long now = System.currentTimeMillis();
        long then = timestamp.getTime();
        Duration duration = Duration.ofMillis(now - then);

        if (duration.toDays() > 0) {
            return Strings.formatTimestamp(timestamp);
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

    record ColorElos(float whiteEloDiff, float blackEloDiff) {}

    private static ColorElos createResultElos(int result, float winElo, float loseElo) {
        switch (result) {
        case WHITE_WIN -> new ColorElos(winElo, loseElo);
        case BLACK_WIN -> new ColorElos(loseElo, winElo);
        case DRAW -> new ColorElos(0, 0);
        }
        throw new IllegalStateException("Invalid result state " + result);
    }
}
