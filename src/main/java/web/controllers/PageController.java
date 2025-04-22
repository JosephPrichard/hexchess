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
import web.reusable.PathService;
import web.reusable.SessionService;
import web.State;
import web.Templates;
import web.views.*;

import java.io.IOException;
import java.util.List;
import java.util.concurrent.CompletableFuture;
import java.util.stream.Collectors;

import static utils.Globals.*;
import static web.reusable.SessionService.*;
import static services.RemoteDict.*;
import static web.Templates.*;

public class PageController extends Jooby {

    public static final int PER_PAGE = 25;

    private final UserDao userDao;
    private final ReplayDao replayDao;
    private final ChallengeDao challengeDao;
    private final RemoteDict remoteDict;
    private final GameService gameService;
    private final SessionService sessionService;
    private final PathService pathService;
    private final Templates templates;
    private final List<String> countryList;

    private String loginHtml;
    private String registerHtml;
    private String defaultHtml;

    public PageController(State state) {
        userDao = state.getUserDao();
        replayDao = state.getReplayDao();
        challengeDao = state.getChallengeDao();
        remoteDict = state.getRemoteDict();
        gameService = state.getGameService();
        sessionService = state.getSessionService();
        pathService = state.getPathService();
        templates = state.getTemplates();
        countryList = state.getCountryList();

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
            Template loginTemplate = templates.getLoginTemplate();
            Template registerTemplate = templates.getRegisterTemplate();
            Template error404Template = templates.getError404Template();

            if (loginTemplate != null) {
                loginHtml = loginTemplate.apply(null);
            }
            if (registerTemplate != null) {
                registerHtml = registerTemplate.apply(null);
            }
            if (error404Template != null) {
                defaultHtml = error404Template.apply(null);
            }
        } catch (IOException ex) {
            LOGGER.error("Failed during page router initialization", ex);
            throw new RuntimeException(ex);
        }

        return this;
    }

    public String sendErrorPage(Context ctx, int code, String message) throws IOException {
        Template template = templates.getErrorTemplate();

        ctx.setResponseCode(code);
        return template.apply(new ErrorPage(code, message));
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

    @Data
    @AllArgsConstructor
    static class IndexPage {
        List<GameView> games;
    }

    public String getIndex(Context ctx) throws IOException {
        Template template = templates.getIndexTemplate();

        List<GameState> gameStates = gameService.getGames(1);

//        List<GameView> gameViews = gameStates.stream().map(GameView::fromGameState).toList();

//        return template.apply(new GamesPage(gameViews));
        return template.apply(null);
    }

    @Data
    @AllArgsConstructor
    static class ProfilePage {
        UserView user;
        List<String> countries;
    }

    public String getProfile(Context ctx) throws IOException {
        Template template = templates.getProfileTemplate();

        SessionValue session = sessionService.parseSession(ctx); // this route is un-cacheable due to using cookies
        if (session == null) {
            return sendErrorPage(ctx, StatusCode.UNAUTHORIZED_CODE, "You must be logged in to access this page.");
        }
        PlayerEntity player = remoteDict.getSession(session.sessionId);
        if (player == null) {
            ctx.setResponseCookie(sessionService.createEmptyCookie());
            return sendErrorPage(ctx, StatusCode.UNAUTHORIZED_CODE, "Session has expired, please login again.");
        }

        UserEntity entity = userDao.getById(player.id);
        UserView view = UserView.create(entity);

        return template.apply(new ProfilePage(view, countryList));
    }

    @Data
    @AllArgsConstructor
    static class LeaderboardPage {
        List<UserView> userList;
        PaginationView pages;
    }

    public String getLeaderboard(Context ctx) throws Exception {
        Template template = templates.getLeaderboardTemplate();

        int page = pathService.getPageParam(ctx);

        Leaderboard leaderboard = remoteDict.getLeaderboardPage(page, PER_PAGE);
        List<UserEntity> entityList = userDao.getByRankedUsers(leaderboard.users);

        RankedUser.joinRanks(leaderboard.users, entityList);

        List<UserView> viewList = entityList.stream().map(UserView::create).toList();

        String resp = template.apply(new LeaderboardPage(viewList, PaginationView.withTotal("?", page, leaderboard.pageCount)));
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
        long userId = pathService.getIdPath(ctx);

        CompletableFuture<UserEntity> userFut = CompletableFuture.supplyAsync(() -> userDao.getById(userId), EXECUTOR);
        CompletableFuture<List<ReplayEntity>> replayListFut =
                CompletableFuture.supplyAsync(() -> replayDao.getUserReplays(userId, null, PER_PAGE), EXECUTOR);

        UserEntity userEntity = userFut.get();
        if (userEntity == null) {
            return sendErrorPage(ctx, StatusCode.NOT_FOUND_CODE, "Couldn't find a user for the provided user id.");
        }

        userEntity.rank = remoteDict.getLeaderboardRank(userEntity.id);
        List<ReplayEntity> replayEntityList = replayListFut.get();

        UserView userView = UserView.create(userEntity);
        List<ReplayView> replayViewList = replayEntityList.stream().map(ReplayView::createRow).collect(Collectors.toList());

        Template template = templates.getUserTemplate();
        String resp = template.apply(new UserPage(userView, replayViewList));
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
        Template template = templates.getSearchTemplate();

        int page = pathService.getPageParam(ctx);

        String name = ctx.query("username").toOptional().orElse("");

        if (name.isEmpty()) {
            return template.apply(new SearchPage(name, List.of(), PaginationView.ofUnlimited("?", page)));
        }

        List<UserEntity> entityList = userDao.searchByName(name, page, PER_PAGE);
        List<UserView> viewList = entityList.stream().map(UserView::create).toList();

        PaginationView pagination = PaginationView.ofUnlimited(String.format("?username=%s&", name), page);
        String resp = template.apply(new SearchPage(name, viewList, pagination));
//            ctx.setResponseHeader("Cache-Control", "max-age=3600, must-revalidate");
        return resp;
    }

    @Data
    @AllArgsConstructor
    static class ReplayPage {
        ReplayView replay;
    }

    public String getGameReplay(Context ctx) throws IOException {
        Template template = templates.getReplayTemplate();

        long replayId = pathService.getIdPath(ctx);

        ReplayEntity entity = replayDao.getReplay(replayId);
        ReplayView view = ReplayView.createHeader(entity);

        String resp = template.apply(new ReplayPage(view));
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
        Template template = templates.getChallengesTemplate();

        String participants = ctx.query("participants").toOptional().orElse("received");

        SessionValue session = sessionService.parseSession(ctx); // this route is un-cacheable due to using cookies
        if (session == null) {
            return sendErrorPage(ctx, StatusCode.UNAUTHORIZED_CODE, "You must be logged in to access this page.");
        }
        PlayerEntity player = remoteDict.getSession(session.sessionId);
        if (player == null) {
            ctx.setResponseCookie(sessionService.createEmptyCookie());
            return sendErrorPage(ctx, StatusCode.UNAUTHORIZED_CODE, "Session has expired, please login again.");
        }

        List<ChallengeEntity> entityList;
        boolean sender;

        switch (participants) {
        case "received":
            entityList = challengeDao.getByParticipant(null, player.id);
            sender = false;
            break;
        case "sent":
            entityList = challengeDao.getByParticipant(player.id, null);
            sender = true;
            break;
        default:
            int code = StatusCode.BAD_REQUEST_CODE;
            ctx.setResponseCode(code);
            return templates.getErrorTemplate().apply(new ErrorPage(code, "Invalid value for participants, must be sent or received."));
        }

        List<ChallengeView> viewList = entityList.stream().map(ChallengeView::create).toList();

        String resp = template.apply(new ChallengesPage(viewList, sender));
//        ctx.setResponseHeader("Cache-Control", "max-age=60, must-revalidate");
        return resp;
    }
}
