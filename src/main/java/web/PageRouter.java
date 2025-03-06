package web;

import domain.ChessBoard;
import io.jooby.*;
import lombok.AllArgsConstructor;
import lombok.Data;
import models.*;

import java.io.IOException;
import java.util.List;
import java.util.concurrent.CompletableFuture;

import static utils.Globals.*;

public class PageRouter extends Jooby {

    public static final int PER_PAGE = 25;

    private final State state;
    private String initialBoardJson;
    private String loginHtml;
    private String registerHtml;

    public PageRouter(State state) {
        this.state = state;

        initStatics(state);

        setWorker(EXECUTOR);

        use(next -> ctx -> {
            ctx.setResponseType(MediaType.HTML);
            return next.apply(ctx);
        });

        get("*", ctx -> {
            var templates = state.getTemplates();
            var message = "Sorry, the page you are looking for does not exist. You might have followed a broken link or entered a URL that doesn't exist on this site.";
            var code = StatusCode.NOT_FOUND_CODE;
            ctx.setResponseCode(code);
            return templates.getErrorTemplate().apply(new ErrorView(code, message));
        });

        get("/", this::getIndex);
        get("/index", this::getIndex);
        get("/play", this::getIndex);
        get("/login", ctx -> loginHtml);
        get("/register", ctx -> registerHtml);
        get("/settings", this::getSettings);
        get("/leaderboard", this::getLeaderboardV1);
        get("/players/{id}", this::getPlayerV1);
        get("/players/search", this::searchPlayers);
        get("/games/histories/{id}", this::getGameHistories);
        get("/challenges", this::getChallenges);
    }

    public void initStatics(State state) {
        try {
            var templates = state.getTemplates();
            var loginTemplate = templates.getLoginTemplate();
            var registerTemplate = templates.getRegisterTemplate();

            initialBoardJson = JSON_MAPPER.writeValueAsString(ChessBoard.initial());
            if (loginTemplate != null) {
                loginHtml = loginTemplate.apply(null);
            }
            if (registerTemplate != null) {
                registerHtml = registerTemplate.apply(null);
            }
        } catch (IOException ex) {
            LOGGER.error("Failed during page router initialization", ex);
            throw new RuntimeException(ex);
        }
    }

    @Data
    @AllArgsConstructor
    public static class ErrorView {
        public int code;
        public String message;
    }

    public String getIndex(Context ctx) throws IOException {
        var gameService = state.getGameService();
        var templates = state.getTemplates();

//        var gamesResult = gameService.getGames(null);
        var template = templates.getIndexTemplate();
        return template.apply(null);
    }

    public String getSettings(Context ctx) throws IOException {
        var templates = state.getTemplates();
        var sessionService = state.getSessionService();
        var userDao = state.getUserDao();

        var cookieStr = ctx.header("Cookie").valueOrNull(); // this route is un-cacheable due to using cookies

        var session = sessionService.getSession(cookieStr);
        if (session == null) {
            var code = StatusCode.UNAUTHORIZED_CODE;
            ctx.setResponseCode(code);
            return templates.getErrorTemplate().apply(new ErrorView(code, "You must be logged in to access this page."));
        }

        var user = userDao.getById(session.getPlayerId());

        var template = templates.getPreferencesTemplates();
        return template.apply(user);
    }

    @Data
    @AllArgsConstructor
    public static class LeaderboardView {
        public List<User> userList;
        public Pagination pages;
    }

    public String getLeaderboardV1(Context ctx) throws Exception {
        var templates = state.getTemplates();
        var userDao = state.getUserDao();

        try {
            int page = ctx.query("page").toOptional().map(Integer::parseUnsignedInt).orElse(1);

            var userListFut = CompletableFuture.supplyAsync(() -> userDao.getLeaderboard(page, PER_PAGE), EXECUTOR);
            var totalPagesFut = CompletableFuture.supplyAsync(() -> userDao.countPages(PER_PAGE), EXECUTOR);
            var userList = userListFut.get();
            var totalPages = totalPagesFut.get();

            userList.forEach(User::sanitize);

            var template = templates.getLeaderboardTemplate();
            return template.apply(new LeaderboardView(userList, Pagination.withTotal("?", page, totalPages)));
        } catch (NumberFormatException ex) {
            var code = StatusCode.BAD_REQUEST_CODE;
            ctx.setResponseCode(code);
            return templates.getErrorTemplate().apply(new ErrorView(code, "Invalid param 'page': must be a positive integer."));
        }
    }

    public String getLeaderboardV2(Context ctx) throws Exception {
        var templates = state.getTemplates();
        var userDao = state.getUserDao();
        var remoteDict = state.getRemoteDict();

        try {
            int page = ctx.query("page").toOptional().map(Integer::parseUnsignedInt).orElse(1);

            var leaderboard = remoteDict.getLeaderboardPage(page, PER_PAGE);
            var userList = userDao.getByRanks(leaderboard.getUsers());

            RankedUser.joinRanks(leaderboard.getUsers(), userList);
            userList.forEach(User::sanitize);

            var template = templates.getLeaderboardTemplate();
            return template.apply(new LeaderboardView(userList, Pagination.withTotal("?", page, leaderboard.getPageCount())));
        } catch (NumberFormatException ex) {
            var code = StatusCode.BAD_REQUEST_CODE;
            ctx.setResponseCode(code);
            var template = templates.getErrorTemplate();
            return template.apply(new ErrorView(code, "Invalid param 'page': must be a positive integer."));
        }
    }

    @Data
    @AllArgsConstructor
    public static class ProfileView {
        public User user;
        public List<History> historyList;
    }

    public String getPlayerV1(Context ctx) throws Exception {
        var templates = state.getTemplates();
        var userDao = state.getUserDao();
        var historyDao = state.getHistoryDao();

        var userIdSlug = ctx.path("id");
        if (userIdSlug.isMissing()) {
            var code = StatusCode.BAD_REQUEST_CODE;
            ctx.setResponseCode(code);
            return templates.getErrorTemplate().apply(new ErrorView(code, "Invalid param 'id': must contain id within slug."));
        }
        var userId = userIdSlug.toString();

        var userFut = CompletableFuture.supplyAsync(() -> userDao.getByIdWithRank(userId), EXECUTOR);
        var historyListFut = CompletableFuture.supplyAsync(() -> historyDao.getUserHistories(userId, null, 25), EXECUTOR);
        var user = userFut.get();
        var historyList = historyListFut.get();

        user.sanitize();
        historyList.forEach(History::sanitize);

        var template = templates.getProfileTemplate();
        return template.apply(new ProfileView(user, historyList));
    }

    public String getPlayerV2(Context ctx) throws Exception {
        var templates = state.getTemplates();
        var userDao = state.getUserDao();
        var historyDao = state.getHistoryDao();
        var remoteDict = state.getRemoteDict();

        var userIdSlug = ctx.path("id");
        if (userIdSlug.isMissing()) {
            var code = StatusCode.BAD_REQUEST_CODE;
            ctx.setResponseCode(code);
            var template = templates.getErrorTemplate();
            return template.apply(new ErrorView(code, "Invalid param 'id': must contain id within slug."));
        }
        var userId = userIdSlug.toString();

        var userFut = CompletableFuture.supplyAsync(() -> userDao.getById(userId), EXECUTOR);
        var historyListFut = CompletableFuture.supplyAsync(() -> historyDao.getUserHistories(userId, null, 10), EXECUTOR);

        var user = userFut.get();
        var rank = remoteDict.getLeaderboardRank(user.getId());
        user.setRank(rank);
        var historyList = historyListFut.get();

        user.sanitize();
        historyList.forEach(History::sanitize);

        var template = templates.getProfileTemplate();
        return template.apply(new ProfileView(user, historyList));
    }

    @Data
    @AllArgsConstructor
    public static class SearchView {
        public String searchText;
        public List<User> userList;
        public Pagination pages;
    }

    public String searchPlayers(Context ctx) throws IOException {
        var templates = state.getTemplates();
        var userDao = state.getUserDao();

        try {
            int page = ctx.query("page").toOptional().map(Integer::parseUnsignedInt).orElse(1);
            var name = ctx.query("username").toOptional().orElse("");

            var template = templates.getSearchTemplate();
            if (name.isEmpty()) {
                return template.apply(new SearchView(name, List.of(), Pagination.ofUnlimited("?", page)));
            }

            var userList = userDao.searchByName(name, page, 20);
            userList.forEach(User::sanitize);

            var pagination = Pagination.ofUnlimited(String.format("?username=%s&", name), page);
            return template.apply(new SearchView(name, userList, pagination));
        } catch (NumberFormatException ex) {
            var code = StatusCode.BAD_REQUEST_CODE;
            ctx.setResponseCode(code);
            return templates.getErrorTemplate().apply(new ErrorView(code, "Invalid param 'page': must be a positive integer."));
        }
    }

    @Data
    @AllArgsConstructor
    public static class ReplayView {
        public String initialBoard;
        public History history;
    }

    public String getGameHistories(Context ctx) throws IOException {
        var templates = state.getTemplates();
        var historyDao = state.getHistoryDao();

        var historyIdSlug = ctx.path("id");
        if (historyIdSlug.isMissing()) {
            var code = StatusCode.BAD_REQUEST_CODE;
            ctx.setResponseCode(code);
            var template = templates.getErrorTemplate();
            return template.apply(new ErrorView(code, "Invalid param 'id': must contain id within slug."));
        }
        var historyId = Long.parseUnsignedLong(historyIdSlug.toString());

        var history = historyDao.getHistory(historyId);

        history.sanitize();

        var template = templates.getGameStateoryTemplate();
        return template.apply(new ReplayView(initialBoardJson, history));
    }

    @Data
    @AllArgsConstructor
    public static class ChallengeView {
        public List<Challenge> challengeList;
        public boolean areSent;
    }

    public String getChallenges(Context ctx) throws IOException {
        var templates = state.getTemplates();
        var sessionService = state.getSessionService();
        var challengeDao = state.getChallengeDao();

        var participants = ctx.query("participants").toOptional().orElse("received");
        var cookieStr = ctx.header("Cookie").valueOrNull(); // this route is un-cacheable due to using cookies

        var session = sessionService.getSession(cookieStr);
        if (session == null) {
            var code = StatusCode.UNAUTHORIZED_CODE;
            ctx.setResponseCode(code);
            return templates.getErrorTemplate().apply(new ErrorView(code, "You must be logged in to access this page."));
        }

        List<Challenge> challengeList;
        boolean areSent;

        switch (participants) {
            case "received": {
                challengeList = challengeDao.getByParticipant(null, session.getPlayerId());
                areSent = false;
                break;
            }
            case "sent": {
                challengeList = challengeDao.getByParticipant(session.getPlayerId(), null);
                areSent = true;
                break;
            }
            default: {
                var code = StatusCode.BAD_REQUEST_CODE;
                ctx.setResponseCode(code);
                return templates.getErrorTemplate().apply(new ErrorView(code, "Invalid value for participants, must be 'sent' or 'received'."));
            }
        }

        EXECUTOR.execute(() -> challengeDao.deleteExpired(session.getPlayerId()));

        var template = templates.getChallengesTemplate();
        return template.apply(new ChallengeView(challengeList, areSent));
    }
}
