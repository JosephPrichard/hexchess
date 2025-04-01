package web;

import com.github.jknack.handlebars.Template;
import dao.ReplayDao;
import io.jooby.*;
import models.ReplayEntity;

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

        get("/partials/player/replays", this::getPlayerReplays);
    }

    public String getPlayerReplays(Context ctx) throws Exception {
        ReplayDao replayDao = state.getReplayDao();
        Templates templates = state.getTemplates();

        ctx.setResponseType(MediaType.HTML);

        long userId = ctx.query("userId").longValue();
        Long afterId = ctx.query("afterId").toOptional().map(Long::parseUnsignedLong).orElse(null);

        List<ReplayEntity> replayList = replayDao.getUserReplays(userId, afterId, 25);
        if (replayList.isEmpty()) {
            ctx.setResponseCode(StatusCode.NOT_FOUND_CODE);
            return "";
        }

        replayList.forEach(ReplayEntity::sanitize);

        Template template = templates.getReplayListTemplate();
        String resp = template.apply(replayList);

//        ctx.setResponseHeader("Cache-Control", "max-age=60, must-revalidate");
        return resp;
    }
}
