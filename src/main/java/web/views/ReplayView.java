package web.views;

import lombok.Data;
import org.jsoup.Jsoup;
import models.ReplayEntity;
import web.Constants;

import java.time.format.DateTimeFormatter;

import static utils.Globals.HTML_SAFELIST;

@Data
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
    public String moveList;
    public String playedOn;
    public String result;
    public String whiteEloDiff;
    public String blackEloDiff;
    public String whiteEloColor;
    public String blackEloColor;

    public static ReplayView fromEntity(ReplayEntity entity) {
        ReplayView view = new ReplayView();
        view.id = entity.id;
        view.whiteId = entity.whiteId;
        view.blackId = entity.blackId;
        view.whiteName = Jsoup.clean(entity.whiteName, HTML_SAFELIST);
        view.blackName = Jsoup.clean(entity.blackName, HTML_SAFELIST);
        view.whiteCountry = Jsoup.clean(entity.whiteCountry, HTML_SAFELIST);
        view.blackCountry = Jsoup.clean(entity.blackCountry, HTML_SAFELIST);
        view.winElo = entity.winElo;
        view.loseElo = entity.loseElo;
        view.moveList = entity.moveList;
        view.playedOn = entity.playedOn.toLocalDateTime().format(DATE_FORMATTER);
        view.result = formatResult(entity.result);
        view.whiteEloDiff = getWhiteEloDiff(entity.result, entity.winElo, entity.loseElo);
        view.blackEloDiff = getBlackEloDiff(entity.result, entity.winElo, entity.loseElo);
        view.whiteEloColor = getWhiteEloColor(entity.result);
        view.blackEloColor = getWhiteEloColor(entity.result);
        return view;
    }

    public static String formatResult(int result) {
        return switch (result) {
            case WHITE_WIN -> "White Victory";
            case BLACK_WIN -> "Black Victory";
            case DRAW -> "Draw";
            default -> throw new IllegalStateException("Invalid result state " + result);
        };
    }

    private static String formatElo(float elo) {
        return (elo >= 0 ? "+" : "") + elo;
    }

    public static String getWhiteEloDiff(int result, float winElo, float loseElo) {
        float elo = switch (result) {
            case WHITE_WIN -> winElo;
            case BLACK_WIN -> loseElo;
            case DRAW -> 0;
            default -> throw new IllegalStateException("Invalid result state " + result);
        };
        return formatElo(elo);
    }

    public static String getBlackEloDiff(int result, float winElo, float loseElo) {
        return getWhiteEloDiff(result, loseElo, winElo);
    }

    public static String getWhiteEloColor(int result) {
        return switch (result) {
            case WHITE_WIN -> Constants.GREEN_COLOR;
            case BLACK_WIN -> Constants.RED_COLOR;
            case DRAW -> Constants.YELLOW_COLOR;
            default -> throw new IllegalStateException("Invalid result state " + result);
        };
    }

    public static String getBlackEloColor(int result) {
        return switch (result) {
            case WHITE_WIN -> Constants.RED_COLOR;
            case BLACK_WIN -> Constants.GREEN_COLOR;
            case DRAW -> Constants.YELLOW_COLOR;
            default -> throw new IllegalStateException("Invalid result state " + result);
        };
    }
}
