package web;

import com.github.jknack.handlebars.Template;
import dao.ChallengeDao;
import dao.ReplayDao;
import dao.UserDao;
import domain.ChessBoard;
import services.GameService;
import services.RemoteDict;
import io.jooby.*;
import lombok.AllArgsConstructor;
import lombok.Data;
import models.*;

import java.io.IOException;
import java.util.List;
import java.util.concurrent.CompletableFuture;

import static utils.Globals.*;
import static web.SessionService.*;
import static services.RemoteDict.*;

public class PageRouter extends Jooby {

    public static final int PER_PAGE = 25;
    public static final String MESSAGE_404 =
        "Sorry, the page you are looking for does not exist. " +
        "You might have followed a broken link or entered a URL that doesn't exist on this site.";

    private final State state;
    private String initialBoardJson;
    private String loginHtml;
    private String registerHtml;
    private String defaultHtml;

    public PageRouter(State state) {
        this.state = state;

        setWorker(EXECUTOR);

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
        get("/leaderboard", this::getLeaderboardRedis);
        get("/players/{id}", this::getPlayerRedis);
        get("/players/search", this::searchPlayers);
        get("/games/replay/{id}", this::getGameReplay);
        get("/challenges", this::getChallenges);
    }

    public PageRouter initStatics() {
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
                defaultHtml = errorTemplate.apply(new ErrorView(StatusCode.NOT_FOUND_CODE, MESSAGE_404));
            }
        } catch (IOException ex) {
            LOGGER.error("Failed during page router initialization", ex);
            throw new RuntimeException(ex);
        }

        return this;
    }

    public String sendErrorView(Context ctx, int code, String message) throws IOException {
        Templates templates = state.getTemplates();
        Template template = templates.getErrorTemplate();

        ctx.setResponseCode(code);
        return template.apply(new ErrorView(code, message));
    }

    @Data
    @AllArgsConstructor
    public static class ErrorView {
        public int code;
        public String message;
    }

    public String getIndex(Context ctx) throws IOException {
        GameService gameService = state.getGameService();
        Templates templates = state.getTemplates();

        GetGamesResult gamesResult = gameService.getGames(null);
        Template template = templates.getIndexTemplate();
        return template.apply(null);
    }

    @Data
    @AllArgsConstructor
    public static class ProfileView {
        public UserEntity user;
        public List<String> countries;
    }

    public String getProfile(Context ctx) throws IOException {
        Templates templates = state.getTemplates();
        SessionService sessionService = state.getSessionService();
        UserDao userDao = state.getUserDao();
        RemoteDict remoteDict = state.getRemoteDict();

        SessionValue session = sessionService.parseSession(ctx); // this route is un-cacheable due to using cookies
        if (session == null) {
            return sendErrorView(ctx, StatusCode.UNAUTHORIZED_CODE, "You must be logged in to access this page.");
        }
        PlayerEntity player = remoteDict.getSession(session.getSessionId());
        if (player == null) {
            ctx.setResponseCookie(sessionService.createEmptyCookie());
            return sendErrorView(ctx, StatusCode.UNAUTHORIZED_CODE, "Session has expired, please login again.");
        }

        UserEntity user = userDao.getById(player.getId());

        Template template = templates.getProfileTemplate();
        return template.apply(new ProfileView(user, state.getCountryList()));
    }

    @Data
    @AllArgsConstructor
    public static class LeaderboardView {
        public List<UserEntity> userList;
        public Pagination pages;
    }

    public String getLeaderboardRedis(Context ctx) throws Exception {
        Templates templates = state.getTemplates();
        UserDao userDao = state.getUserDao();
        RemoteDict remoteDict = state.getRemoteDict();

        int page;
        try {
            page = ctx.query("page").toOptional().map(Integer::parseUnsignedInt).orElse(1);
        } catch (NumberFormatException ex) {
            return sendErrorView(ctx, StatusCode.BAD_REQUEST_CODE, "Invalid param 'page': must be a positive integer.");
        }

        RemoteDict.Leaderboard leaderboard = remoteDict.getLeaderboardPage(page, PER_PAGE);
        List<UserEntity> userList = userDao.getByRankedUsers(leaderboard.getUsers());

        RankedUser.joinRanks(leaderboard.getUsers(), userList);
        userList.forEach(UserEntity::sanitize);

        Template template = templates.getLeaderboardTemplate();
        String resp = template.apply(new LeaderboardView(userList, Pagination.withTotal("?", page, leaderboard.getPageCount())));

//            ctx.setResponseHeader("Cache-Control", "max-age=60, must-revalidate");
        return resp;
    }

    @Data
    @AllArgsConstructor
    public static class UserView {
        public UserEntity user;
        public List<ReplayEntity> replayList;
    }

    public String getPlayerRedis(Context ctx) throws Exception {
        Templates templates = state.getTemplates();
        UserDao userDao = state.getUserDao();
        ReplayDao replayDao = state.getReplayDao();
        RemoteDict remoteDict = state.getRemoteDict();

        Value userIdSlug = ctx.path("id");
        if (userIdSlug.isMissing()) {
            return sendErrorView(ctx, StatusCode.BAD_REQUEST_CODE, "Invalid param 'id': must contain id within slug.");
        }
        long userId = userIdSlug.longValue();

        CompletableFuture<UserEntity> userFut = CompletableFuture.supplyAsync(() -> userDao.getById(userId), EXECUTOR);
        CompletableFuture<List<ReplayEntity>> replayListFut =
                CompletableFuture.supplyAsync(() -> replayDao.getUserReplays(userId, null, PER_PAGE), EXECUTOR);

        UserEntity user = userFut.get();
        if (user == null) {
            return sendErrorView(ctx, StatusCode.NOT_FOUND_CODE,"Couldn't find a user for the provided user id.");
        }

        int rank = remoteDict.getLeaderboardRank(user.getId());
        user.setRank(rank);
        List<ReplayEntity> replayList = replayListFut.get();

        user.sanitize();
        replayList.forEach(ReplayEntity::sanitize);

        Template template = templates.getUserTemplate();
        String resp = template.apply(new UserView(user, replayList));

//        ctx.setResponseHeader("Cache-Control", "max-age=60, must-revalidate");
        return resp;
    }

    @Data
    @AllArgsConstructor
    public static class SearchView {
        public String searchText;
        public List<UserEntity> userList;
        public Pagination pages;
    }

    public String searchPlayers(Context ctx) throws IOException {
        Templates templates = state.getTemplates();
        UserDao userDao = state.getUserDao();

        int page;
        try {
           page = ctx.query("page").toOptional().map(Integer::parseUnsignedInt).orElse(1);
        } catch (NumberFormatException ex) {
            return sendErrorView(ctx, StatusCode.BAD_REQUEST_CODE, "Invalid param 'page': must be a positive integer.");
        }

        String name = ctx.query("username").toOptional().orElse("");

        Template template = templates.getSearchTemplate();
        if (name.isEmpty()) {
            return template.apply(new SearchView(name, List.of(), Pagination.ofUnlimited("?", page)));
        }

        List<UserEntity> userList = userDao.searchByName(name, page, PER_PAGE);
        userList.forEach(UserEntity::sanitize);

        Pagination pagination = Pagination.ofUnlimited(String.format("?username=%s&", name), page);
        String resp = template.apply(new SearchView(name, userList, pagination));

//            ctx.setResponseHeader("Cache-Control", "max-age=3600, must-revalidate");
        return resp;
    }

    @Data
    @AllArgsConstructor
    public static class ReplayView {
        public String initialBoard;
        public ReplayEntity replay;
    }

    public String getGameReplay(Context ctx) throws IOException {
        Templates templates = state.getTemplates();
        ReplayDao replayDao = state.getReplayDao();

        Value replayIdSlug = ctx.path("id");
        if (replayIdSlug.isMissing()) {
            return sendErrorView(ctx, StatusCode.BAD_REQUEST_CODE, "Invalid param 'id': must contain id within slug.");
        }
        long replayId = Long.parseUnsignedLong(replayIdSlug.toString());

        ReplayEntity replay = replayDao.getReplay(replayId);
        replay.sanitize();

        Template template = templates.getReplayTemplate();
        String resp = template.apply(new ReplayView(initialBoardJson, replay));

//        ctx.setResponseHeader("Cache-Control", "max-age=86400, must-revalidate"); // this is never updated, we can cache aggressively
        return resp;
    }

    @Data
    @AllArgsConstructor
    public static class ChallengeView {
        public List<ChallengeEntity> challengeList;
        public boolean sender;
    }

    public String getChallenges(Context ctx) throws IOException {
        Templates templates = state.getTemplates();
        SessionService sessionService = state.getSessionService();
        ChallengeDao challengeDao = state.getChallengeDao();
        RemoteDict remoteDict = state.getRemoteDict();

        String participants = ctx.query("participants").toOptional().orElse("received");

        SessionValue session = sessionService.parseSession(ctx); // this route is un-cacheable due to using cookies
        if (session == null) {
            return sendErrorView(ctx, StatusCode.UNAUTHORIZED_CODE, "You must be logged in to access this page.");
        }
        PlayerEntity player = remoteDict.getSession(session.getSessionId());
        if (player == null) {
            ctx.setResponseCookie(sessionService.createEmptyCookie());
            return sendErrorView(ctx, StatusCode.UNAUTHORIZED_CODE, "Session has expired, please login again.");
        }

        List<ChallengeEntity> challengeList;
        boolean sender;

        switch (participants) {
        case "received":
            challengeList = challengeDao.getByParticipant(null, player.getId());
            sender = false;
            break;
        case "sent":
            challengeList = challengeDao.getByParticipant(player.getId(), null);
            sender = true;
            break;
        default:
            int code = StatusCode.BAD_REQUEST_CODE;
            ctx.setResponseCode(code);
            return templates.getErrorTemplate().apply(new ErrorView(code, "Invalid value for participants, must be 'sent' or 'received'."));
        }

        EXECUTOR.execute(() -> challengeDao.deleteExpired(session.getUserId()));

        Template template = templates.getChallengesTemplate();
        String resp = template.apply(new ChallengeView(challengeList, sender));

//        ctx.setResponseHeader("Cache-Control", "max-age=60, must-revalidate");
        return resp;
    }
}
