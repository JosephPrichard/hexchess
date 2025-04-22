package web.controllers;

import chess.ChessBoard;
import io.jooby.Jooby;
import io.jooby.MediaType;
import lombok.SneakyThrows;

import java.time.Duration;

import static utils.Globals.JSON_MAPPER;

public class StaticsController extends Jooby {

    private final String initialBoardJson;

    @SneakyThrows
    public StaticsController() {
        initialBoardJson = JSON_MAPPER.writeValueAsString(ChessBoard.initial());

        assets("/favicon.ico", "/static/images/pieces/white-queen.png");
        assets("/static/*", "static").setMaxAge(Duration.ofHours(1));

        get("/static/initial-board", (ctx) -> {
            ctx.setResponseType(MediaType.JSON);
            ctx.setResponseHeader("Cache-Control", "max-age=3600, must-revalidate");
            return initialBoardJson;
        });
    }
}
