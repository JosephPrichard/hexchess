package web.controllers;

import com.github.jknack.handlebars.Template;
import daos.ChallengeDao;
import daos.ReplayDao;
import daos.UserDao;
import chess.ChessBoard;
import models.*;
import services.GameService;
import services.RemoteDict;
import io.jooby.*;
import lombok.AllArgsConstructor;
import lombok.Data;
import services.SessionService;
import web.State;
import web.Templates;
import web.views.ChallengeView;
import web.views.PaginationView;
import web.views.ReplayView;
import web.views.UserView;

import java.io.IOException;
import java.util.List;
import java.util.concurrent.CompletableFuture;
import java.util.stream.Collectors;

import static utils.Globals.*;
import static services.SessionService.*;
import static services.RemoteDict.*;

public class PageController extends Jooby {

    public static final int PER_PAGE = 25;
    public static final String MESSAGE_404 =
        "Sorry, the page you are looking for does not exist. " +
        "You might have followed a broken link or entered a URL that doesn't exist on this site.";

    private final State state;
    private String initialBoardJson;
    private String loginHtml;
    private String registerHtml;
    private String defaultHtml;

    public PageController(State state) {
        this.state = state;

        setWorker(EXECUTOR);

        error(this::handleError);

        use(next -> ctx -> {
            ctx.setResponseType(MediaType.HTML);
            return next.apply(ctx);
        });

        get("*", ctx -> {
            ctx.setResponseCode(StatusCode.NOT_FOUND_CODE);
            return defaultHtml;
        });

        get("/", this::getIndex);
        get("/index", this::getIndex);
        get("/play", this::getIndex);
        get("/login", ctx -> loginHtml);
        get("/register", ctx -> registerHtml);
        get("/profile", this::getProfile);
        get("/leaderboard", this::getLeaderboard);
        get("/players/{id}", this::getPlayer);
        get("/players/search", this::searchPlayers);
        get("/games/replay/{id}", this::getGameReplay);
        get("/challenges", this::getChallenges);
    }

    public PageController initStatics() {
        try {
            Templates templates = state.getTemplates();
            Template loginTemplate = templates.getLoginTemplate();
            Template registerTemplate = templates.getRegisterTemplate();
            Template errorTemplate = templates.getErrorTemplate();

            initialBoardJson = JSON_MAPPER.writeValueAsString(ChessBoard.initial());
            if (loginTemplate != null) {
                loginHtml = loginTemplate.apply(null);
            }
            if (registerTemplate != null) {
                registerHtml = registerTemplate.apply(null);
            }
            if (errorTemplate != null) {
                defaultHtml = errorTemplate.apply(new ErrorPage(StatusCode.NOT_FOUND_CODE, MESSAGE_404));
            }
        } catch (IOException ex) {
            LOGGER.error("Failed during page router initialization", ex);
            throw new RuntimeException(ex);
        }

        return this;
    }

    public String sendErrorPage(Context ctx, int code, String message) throws IOException {
        Templates templates = state.getTemplates();
        Template template = templates.getErrorTemplate();

        ctx.setResponseCode(code);
        return template.apply(new ErrorPage(code, message));
    }

    @Data
    @AllArgsConstructor
    static class ErrorPage {
        int code;
        String message;
    }

    public void handleError(Context ctx, Throwable cause, StatusCode statusCode) {
        try {
            ctx.setResponseCode(statusCode);
            if (statusCode.value() == 500) {
                LOGGER.error("Error: {}", statusCode, cause);
                ctx.send(sendErrorPage(ctx, StatusCode.SERVER_ERROR_CODE, "Internal server error."));
            } else {
                String message = "Error: " + statusCode + ", " + cause.getMessage();
                LOGGER.error(message);
                ctx.send(sendErrorPage(ctx, StatusCode.UNAUTHORIZED_CODE, "You must be logged in to access this page."));
            }
        } catch (IOException e) {
            LOGGER.error("Error:", e);
            ctx.setResponseType(MediaType.TEXT);
            ctx.send("Internal Server Error");
        }
    }

    public String getIndex(Context ctx) throws IOException {
        GameService gameService = state.getGameService();
        Templates templates = state.getTemplates();

        GetGamesResult gamesResult = gameService.getGames(null);

        Template template = templates.getIndexTemplate();
        String resp = template.apply(null);

        ctx.setResponseType(MediaType.HTML);
        return resp;
    }

    @Data
    @AllArgsConstructor
    static class ProfilePage {
        UserView user;
        List<String> countries;
    }

    public String getProfile(Context ctx) throws IOException {
        Templates templates = state.getTemplates();
        SessionService sessionService = state.getSessionService();
        UserDao userDao = state.getUserDao();
        RemoteDict remoteDict = state.getRemoteDict();

        SessionValue session = sessionService.parseSession(ctx); // this route is un-cacheable due to using cookies
        if (session == null) {
            return sendErrorPage(ctx, StatusCode.UNAUTHORIZED_CODE, "You must be logged in to access this page.");
        }
        PlayerEntity player = remoteDict.getSession(session.getSessionId());
        if (player == null) {
            ctx.setResponseCookie(sessionService.createEmptyCookie());
            return sendErrorPage(ctx, StatusCode.UNAUTHORIZED_CODE, "Session has expired, please login again.");
        }

        UserEntity entity = userDao.getById(player.getId());
        UserView view = UserView.fromEntity(entity);

        Template template = templates.getProfileTemplate();
        String resp = template.apply(new ProfilePage(view, state.getCountryList()));

        ctx.setResponseType(MediaType.HTML);
        return resp;
    }

    @Data
    @AllArgsConstructor
    static class LeaderboardPage {
        List<UserView> userList;
        PaginationView pages;
    }

    public String getLeaderboard(Context ctx) throws Exception {
        Templates templates = state.getTemplates();
        UserDao userDao = state.getUserDao();
        RemoteDict remoteDict = state.getRemoteDict();

        int page;
        try {
            page = ctx.query("page").toOptional().map(Integer::parseUnsignedInt).orElse(1);
        } catch (NumberFormatException ex) {
            return sendErrorPage(ctx, StatusCode.BAD_REQUEST_CODE, "Invalid param page: must be a positive integer.");
        }

        RemoteDict.Leaderboard leaderboard = remoteDict.getLeaderboardPage(page, PER_PAGE);
        List<UserEntity> entityList = userDao.getByRankedUsers(leaderboard.getUsers());

        RankedUser.joinRanks(leaderboard.getUsers(), entityList);

        List<UserView> viewList = entityList.stream().map(UserView::fromEntity).toList();

        Template template = templates.getLeaderboardTemplate();
        String resp = template.apply(new LeaderboardPage(viewList, PaginationView.withTotal("?", page, leaderboard.getPageCount())));

        ctx.setResponseType(MediaType.HTML);
//            ctx.setResponseHeader("Cache-Control", "max-age=60, must-revalidate");
        return resp;
    }

    @Data
    @AllArgsConstructor
    static class UserPage {
        UserView user;
        List<ReplayView> replayList;
    }

    public String getPlayer(Context ctx) throws Exception {
        Templates templates = state.getTemplates();
        UserDao userDao = state.getUserDao();
        ReplayDao replayDao = state.getReplayDao();
        RemoteDict remoteDict = state.getRemoteDict();

        long userId;
        try {
            Value userIdPath = ctx.path("id");
            if (userIdPath.isMissing()) {
                return sendErrorPage(ctx, StatusCode.BAD_REQUEST_CODE, "Invalid param id: must contain id within path parameter.");
            }
            userId = Long.parseUnsignedLong(userIdPath.value());
        } catch (NumberFormatException ex) {
            return sendErrorPage(ctx, StatusCode.BAD_REQUEST_CODE, "Invalid param id: must be a positive integer.");
        }

        CompletableFuture<UserEntity> userFut = CompletableFuture.supplyAsync(() -> userDao.getById(userId), EXECUTOR);
        CompletableFuture<List<ReplayEntity>> replayListFut =
                CompletableFuture.supplyAsync(() -> replayDao.getUserReplays(userId, null, PER_PAGE), EXECUTOR);

        UserEntity userEntity = userFut.get();
        if (userEntity == null) {
            return sendErrorPage(ctx, StatusCode.NOT_FOUND_CODE,"Couldn't find a user for the provided user id.");
        }

        int rank = remoteDict.getLeaderboardRank(userEntity.getId());
        userEntity.setRank(rank);
        List<ReplayEntity> replayEntityList = replayListFut.get();

        UserView userView = UserView.fromEntity(userEntity);
        List<ReplayView> replayViewList = replayEntityList.stream().map(ReplayView::fromEntity).collect(Collectors.toList());

        Template template = templates.getUserTemplate();
        String resp = template.apply(new UserPage(userView, replayViewList));

        ctx.setResponseType(MediaType.HTML);
//        ctx.setResponseHeader("Cache-Control", "max-age=60, must-revalidate");
        return resp;
    }

    @Data
    @AllArgsConstructor
    static class SearchPage {
        String searchText;
        List<UserView> userList;
        PaginationView pages;
    }

    public String searchPlayers(Context ctx) throws IOException {
        Templates templates = state.getTemplates();
        UserDao userDao = state.getUserDao();

        int page;
        try {
           page = ctx.query("page").toOptional().map(Integer::parseUnsignedInt).orElse(1);
        } catch (NumberFormatException ex) {
            return sendErrorPage(ctx, StatusCode.BAD_REQUEST_CODE, "Invalid param page: must be a positive integer.");
        }

        String name = ctx.query("username").toOptional().orElse("");

        Template template = templates.getSearchTemplate();
        if (name.isEmpty()) {
            return template.apply(new SearchPage(name, List.of(), PaginationView.ofUnlimited("?", page)));
        }

        List<UserEntity> entityList = userDao.searchByName(name, page, PER_PAGE);
        List<UserView> viewList = entityList.stream().map(UserView::fromEntity).toList();

        PaginationView pagination = PaginationView.ofUnlimited(String.format("?username=%s&", name), page);
        String resp = template.apply(new SearchPage(name, viewList, pagination));

        ctx.setResponseType(MediaType.HTML);
//            ctx.setResponseHeader("Cache-Control", "max-age=3600, must-revalidate");
        return resp;
    }

    @Data
    @AllArgsConstructor
    static class ReplayPage {
        String initialBoard;
        ReplayView replay;
    }

    public String getGameReplay(Context ctx) throws IOException {
        Templates templates = state.getTemplates();
        ReplayDao replayDao = state.getReplayDao();

        long replayId;
        try {
            Value replayIdPath = ctx.path("id");
            if (replayIdPath.isMissing()) {
                return sendErrorPage(ctx, StatusCode.BAD_REQUEST_CODE, "Invalid param id: must contain id within path parameter.");
            }
            replayId = Long.parseUnsignedLong(replayIdPath.value());
        } catch (NumberFormatException ex) {
            return sendErrorPage(ctx, StatusCode.BAD_REQUEST_CODE, "Invalid param id: must be a positive integer.");
        }

        ReplayEntity entity = replayDao.getReplay(replayId);
        ReplayView view = ReplayView.fromEntity(entity);

        Template template = templates.getReplayTemplate();
        String resp = template.apply(new ReplayPage(initialBoardJson, view));

        ctx.setResponseType(MediaType.HTML);
//        ctx.setResponseHeader("Cache-Control", "max-age=86400, must-revalidate"); // this is never updated, we can cache aggressively
        return resp;
    }

    @Data
    @AllArgsConstructor
    static class ChallengesPage {
        List<ChallengeView> challengeList;
        boolean sender;
    }

    public String getChallenges(Context ctx) throws IOException {
        Templates templates = state.getTemplates();
        SessionService sessionService = state.getSessionService();
        ChallengeDao challengeDao = state.getChallengeDao();
        RemoteDict remoteDict = state.getRemoteDict();

        String participants = ctx.query("participants").toOptional().orElse("received");

        SessionValue session = sessionService.parseSession(ctx); // this route is un-cacheable due to using cookies
        if (session == null) {
            return sendErrorPage(ctx, StatusCode.UNAUTHORIZED_CODE, "You must be logged in to access this page.");
        }
        PlayerEntity player = remoteDict.getSession(session.getSessionId());
        if (player == null) {
            ctx.setResponseCookie(sessionService.createEmptyCookie());
            return sendErrorPage(ctx, StatusCode.UNAUTHORIZED_CODE, "Session has expired, please login again.");
        }

        List<ChallengeEntity> entityList;
        boolean sender;

        switch (participants) {
        case "received":
            entityList = challengeDao.getByParticipant(null, player.getId());
            sender = false;
            break;
        case "sent":
            entityList = challengeDao.getByParticipant(player.getId(), null);
            sender = true;
            break;
        default:
            int code = StatusCode.BAD_REQUEST_CODE;
            ctx.setResponseCode(code);
            return templates.getErrorTemplate().apply(new ErrorPage(code, "Invalid value for participants, must be sent or received."));
        }

        List<ChallengeView> viewList = entityList.stream().map(ChallengeView::fromEntity).toList();

        Template template = templates.getChallengesTemplate();
        String resp = template.apply(new ChallengesPage(viewList, sender));

        ctx.setResponseType(MediaType.HTML);
//        ctx.setResponseHeader("Cache-Control", "max-age=60, must-revalidate");
        return resp;
    }
}
