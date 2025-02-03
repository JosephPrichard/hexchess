package web;

import io.jooby.Jooby;
import io.jooby.MediaType;
import io.jooby.StatusCode;
import models.History;

public class PartialsRouter extends Jooby {

    public PartialsRouter(State state) {
        var historyDao = state.getHistoryDao();
        var templates = state.getTemplates();

        get("/partials/*", ctx -> {
            ctx.setResponseCode(StatusCode.NOT_FOUND_CODE);
            return "";
        });

        get("/partials/player-history", ctx -> {
            ctx.setResponseType(MediaType.HTML);

            var userIdSlug = ctx.query("userId");
            if (userIdSlug.isMissing()) {
                ctx.setResponseCode(StatusCode.BAD_REQUEST_CODE);
                return "";
            }
            var userId = userIdSlug.toString();
            Long afterId = ctx.query("afterId").toOptional().map(Long::parseUnsignedLong).orElse(null);

            var historyList = historyDao.getUserHistories(userId, afterId, 25);
            if (historyList.isEmpty()) {
                ctx.setResponseCode(StatusCode.NOT_FOUND_CODE);
                return "";
            }

            historyList.forEach(History::sanitize);

            var template = templates.getHistoryListTemplate();
            return template.apply(historyList);
        });
    }
}
