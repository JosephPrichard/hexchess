package web.controllers;

import com.github.jknack.handlebars.Template;
import daos.ReplayDao;
import io.jooby.*;
import models.ReplayEntity;
import web.State;
import web.Templates;
import web.views.ReplayView;

import java.util.List;

import static utils.Globals.EXECUTOR;
import static utils.Globals.LOGGER;

public class PartialsController extends Jooby {

    public final State state;

    public PartialsController(State state) {
        this.state = state;

        setWorker(EXECUTOR);

        error(this::handleError);

        get("/partials/*", ctx -> {
            ctx.setResponseCode(StatusCode.NOT_FOUND_CODE);
            return "";
        });

        get("/partials/player/replays", this::getPlayerReplays);
    }

    public void handleError(Context ctx, Throwable cause, StatusCode statusCode) {
        ctx.setResponseCode(statusCode);
        if (statusCode.value() == 500) {
            LOGGER.error("Error: {}", statusCode, cause);
        } else {
            String message = "Error: " + statusCode + ", " + cause.getMessage();
            LOGGER.error(message);
        }
        ctx.send("");
    }

    public String getPlayerReplays(Context ctx) throws Exception {
        ReplayDao replayDao = state.getReplayDao();
        Templates templates = state.getTemplates();

        long userId = ctx.query("userId").longValue();
        Long afterId = ctx.query("afterId").toOptional().map(Long::parseUnsignedLong).orElse(null);

        List<ReplayEntity> entityList = replayDao.getUserReplays(userId, afterId, 25);
        if (entityList.isEmpty()) {
            ctx.setResponseCode(StatusCode.NOT_FOUND_CODE);
            return "";
        }

        List<ReplayView> viewList = entityList.stream().map(ReplayView::createRow).toList();

        Template template = templates.getReplayListTemplate();
        String resp = template.apply(viewList);

        ctx.setResponseType(MediaType.HTML);
//        ctx.setResponseHeader("Cache-Control", "max-age=60, must-revalidate");
        return resp;
    }
}
