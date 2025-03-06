package web;

import io.jooby.Context;
import io.jooby.Jooby;
import io.jooby.MediaType;
import io.jooby.StatusCode;
import lombok.AllArgsConstructor;
import models.History;

import static utils.Globals.EXECUTOR;

public class PartialsRouter extends Jooby {

    public final State state;

    public PartialsRouter(State state) {
        this.state = state;

        setWorker(EXECUTOR);

        get("/partials/*", ctx -> {
            ctx.setResponseCode(StatusCode.NOT_FOUND_CODE);
            return "";
        });

        get("/partials/player-history", this::getPlayerHistory);
    }

    public String getPlayerHistory(Context ctx) throws Exception {
        var historyDao = state.getHistoryDao();
        var templates = state.getTemplates();

        ctx.setResponseType(MediaType.HTML);

        var userIdSlug = ctx.query("userId");
        if (userIdSlug.isMissing()) {
            ctx.setResponseCode(StatusCode.BAD_REQUEST_CODE);
            return "";
        }
        var userId = userIdSlug.toString();
        var afterId = ctx.query("afterId").toOptional().map(Long::parseUnsignedLong).orElse(null);

        var historyList = historyDao.getUserHistories(userId, afterId, 25);
        if (historyList.isEmpty()) {
            ctx.setResponseCode(StatusCode.NOT_FOUND_CODE);
            return "";
        }

        historyList.forEach(History::sanitize);

        var template = templates.getHistoryListTemplate();
        return template.apply(historyList);
    }
}
