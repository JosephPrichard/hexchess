package web;

import com.github.jknack.handlebars.Template;
import dao.HistoryDao;
import io.jooby.*;
import lombok.AllArgsConstructor;
import models.History;

import java.util.List;

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
        HistoryDao historyDao = state.getHistoryDao();
        Templates templates = state.getTemplates();

        ctx.setResponseType(MediaType.HTML);

        ValueNode userIdSlug = ctx.query("userId");
        if (userIdSlug.isMissing()) {
            ctx.setResponseCode(StatusCode.BAD_REQUEST_CODE);
            return "";
        }
        String userId = userIdSlug.toString();
        Long afterId = ctx.query("afterId").toOptional().map(Long::parseUnsignedLong).orElse(null);

        List<History> historyList = historyDao.getUserHistories(userId, afterId, 25);
        if (historyList.isEmpty()) {
            ctx.setResponseCode(StatusCode.NOT_FOUND_CODE);
            return "";
        }

        historyList.forEach(History::sanitize);

        Template template = templates.getHistoryListTemplate();
        String resp = template.apply(historyList);

//        ctx.setResponseHeader("Cache-Control", "max-age=60, must-revalidate");
        return resp;
    }
}
