package models;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;
import org.jsoup.Jsoup;
import utils.Globals;

import java.sql.Timestamp;
import java.time.format.DateTimeFormatter;

import static utils.Globals.HTML_SAFELIST;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class HistoryEntity {
    private static final DateTimeFormatter DATE_FORMATTER = DateTimeFormatter.ofPattern("MM/dd/yyyy");
    public static final int WHITE_WIN = 0;
    public static final int BLACK_WIN = 1;
    public static final int DRAW = 2;

    public long id;
    public String whiteId;
    public String blackId;
    public String whiteName;
    public String blackName;
    public String whiteCountry;
    public String blackCountry;
    public String data; // this is expensive, so for certain views we don't fetch it
    public int result;
    public float winElo;
    public float loseElo;
    @EqualsAndHashCode.Exclude
    public Timestamp playedOn;

    public String getFormattedResult() {
        return switch (result) {
            case WHITE_WIN -> "White Victory";
            case BLACK_WIN -> "Black Victory";
            case DRAW -> "Draw";
            default -> throw new IllegalStateException("Invalid result state " + result);
        };
    }

    private String formatElo(float elo) {
        return (elo >= 0 ? "+" : "") + elo;
    }

    public String getWhiteEloDiff() {
        var elo = switch (result) {
            case WHITE_WIN -> winElo;
            case BLACK_WIN -> loseElo;
            case DRAW -> 0;
            default -> throw new IllegalStateException("Invalid result state " + result);
        };
        return formatElo(elo);
    }

    public String getBlackEloDiff() {
        var elo = switch (result) {
            case WHITE_WIN -> loseElo;
            case BLACK_WIN -> winElo;
            case DRAW -> 0;
            default -> throw new IllegalStateException("Invalid result state " + result);
        };
        return formatElo(elo);
    }

    public String getWhiteEloColor() {
        return switch (result) {
            case WHITE_WIN -> Globals.GREEN_COLOR;
            case BLACK_WIN -> Globals.RED_COLOR;
            case DRAW -> Globals.YELLOW_COLOR;
            default -> throw new IllegalStateException("Invalid result state " + result);
        };
    }

    public String getBlackEloColor() {
        return switch (result) {
            case WHITE_WIN -> Globals.RED_COLOR;
            case BLACK_WIN -> Globals.GREEN_COLOR;
            case DRAW -> Globals.YELLOW_COLOR;
            default -> throw new IllegalStateException("Invalid result state " + result);
        };
    }

    public String getFormattedPlayedOn() {
        return playedOn.toLocalDateTime().format(DATE_FORMATTER);
    }

    public void sanitize() {
        whiteName = Jsoup.clean(whiteName, HTML_SAFELIST);
        blackName = Jsoup.clean(blackName, HTML_SAFELIST);
        whiteCountry = Jsoup.clean(whiteCountry, HTML_SAFELIST);
        blackCountry = Jsoup.clean(blackCountry, HTML_SAFELIST);
        if (data != null) {
            Jsoup.clean(data, HTML_SAFELIST);
        }
    }
}