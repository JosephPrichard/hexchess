package web;

import com.github.jknack.handlebars.Template;
import dao.ChallengeDao;
import dao.HistoryDao;
import dao.UserDao;
import domain.ChessBoard;
import services.GameService;
import services.RemoteDict;
import io.jooby.*;
import io.jooby.exception.StatusCodeException;
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
            Templates templates = state.getTemplates();
            String message = "Sorry, the page you are looking for does not exist. You might have followed a broken link or entered a URL that doesn't exist on this site.";
            int code = StatusCode.NOT_FOUND_CODE;
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
            Templates templates = state.getTemplates();
            Template loginTemplate = templates.getLoginTemplate();
            Template registerTemplate = templates.getRegisterTemplate();

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

    public String getSettings(Context ctx) throws IOException {
        Templates templates = state.getTemplates();
        SessionService sessionService = state.getSessionService();
        UserDao userDao = state.getUserDao();
        RemoteDict remoteDict = state.getRemoteDict();

        SessionValue session = sessionService.parseSession(ctx); // this route is un-cacheable due to using cookies
        if (session == null) {
            return sendErrorView(ctx, StatusCode.UNAUTHORIZED_CODE, "You must be logged in to access this page.");
        }
        Player player = remoteDict.getSession(session.getSessionId());
        if (player == null) {
            return sendErrorView(ctx, StatusCode.UNAUTHORIZED_CODE, "Session has expired, please login again.");
        }

        User user = userDao.getById(player.getId());

        Template template = templates.getPreferencesTemplates();
        return template.apply(user);
    }

    @Data
    @AllArgsConstructor
    public static class LeaderboardView {
        public List<User> userList;
        public Pagination pages;
    }

    public String getLeaderboardV1(Context ctx) throws Exception {
        Templates templates = state.getTemplates();
        UserDao userDao = state.getUserDao();

        try {
            int page = ctx.query("page").toOptional().map(Integer::parseUnsignedInt).orElse(1);

            CompletableFuture<List<User>> userListFut = CompletableFuture.supplyAsync(() -> userDao.getLeaderboard(page, PER_PAGE), EXECUTOR);
            CompletableFuture<Integer> totalPagesFut = CompletableFuture.supplyAsync(() -> userDao.countPages(PER_PAGE), EXECUTOR);
            List<User> userList = userListFut.get();
            Integer totalPages = totalPagesFut.get();

            userList.forEach(User::sanitize);

            Template template = templates.getLeaderboardTemplate();
            String resp = template.apply(new LeaderboardView(userList, Pagination.withTotal("?", page, totalPages)));

//            ctx.setResponseHeader("Cache-Control", "max-age=60, must-revalidate");
            return resp;
        } catch (NumberFormatException ex) {
            return sendErrorView(ctx, StatusCode.BAD_REQUEST_CODE, "Invalid param 'page': must be a positive integer.");
        }
    }

    public String getLeaderboardV2(Context ctx) throws Exception {
        Templates templates = state.getTemplates();
        UserDao userDao = state.getUserDao();
        RemoteDict remoteDict = state.getRemoteDict();

        try {
            int page = ctx.query("page").toOptional().map(Integer::parseUnsignedInt).orElse(1);

            RemoteDict.Leaderboard leaderboard = remoteDict.getLeaderboardPage(page, PER_PAGE);
            List<User> userList = userDao.getByRanks(leaderboard.getUsers());

            RankedUser.joinRanks(leaderboard.getUsers(), userList);
            userList.forEach(User::sanitize);

            Template template = templates.getLeaderboardTemplate();
            String resp = template.apply(new LeaderboardView(userList, Pagination.withTotal("?", page, leaderboard.getPageCount())));

//            ctx.setResponseHeader("Cache-Control", "max-age=60, must-revalidate");
            return resp;
        } catch (NumberFormatException ex) {
            return sendErrorView(ctx, StatusCode.BAD_REQUEST_CODE, "Invalid param 'page': must be a positive integer.");
        }
    }

    @Data
    @AllArgsConstructor
    public static class ProfileView {
        public User user;
        public List<History> historyList;
    }

    public String getPlayerV1(Context ctx) throws Exception {
        Templates templates = state.getTemplates();
        UserDao userDao = state.getUserDao();
        HistoryDao historyDao = state.getHistoryDao();

        Value userIdSlug = ctx.path("id");
        if (userIdSlug.isMissing()) {
            return sendErrorView(ctx, StatusCode.BAD_REQUEST_CODE, "Invalid param 'id': must contain id within slug.");
        }
        String userId = userIdSlug.toString();

        CompletableFuture<User> userFut = CompletableFuture.supplyAsync(() -> userDao.getByIdWithRank(userId), EXECUTOR);
        CompletableFuture<List<History>> historyListFut =
                CompletableFuture.supplyAsync(() -> historyDao.getUserHistories(userId, null, 25), EXECUTOR);
        User user = userFut.get();
        List<History> historyList = historyListFut.get();

        if (user == null) {
            return sendErrorView(ctx, StatusCode.NOT_FOUND_CODE,  "Couldn't find a user for the provided user id.");
        }

        user.sanitize();
        historyList.forEach(History::sanitize);

        Template template = templates.getProfileTemplate();
        String resp = template.apply(new ProfileView(user, historyList));

//        ctx.setResponseHeader("Cache-Control", "max-age=60, must-revalidate");
        return resp;
    }

    public String getPlayerV2(Context ctx) throws Exception {
        Templates templates = state.getTemplates();
        UserDao userDao = state.getUserDao();
        HistoryDao historyDao = state.getHistoryDao();
        RemoteDict remoteDict = state.getRemoteDict();

        Value userIdSlug = ctx.path("id");
        if (userIdSlug.isMissing()) {
            return sendErrorView(ctx, StatusCode.BAD_REQUEST_CODE, "Invalid param 'id': must contain id within slug.");
        }
        String userId = userIdSlug.toString();

        CompletableFuture<User> userFut = CompletableFuture.supplyAsync(() -> userDao.getById(userId), EXECUTOR);
        CompletableFuture<List<History>> historyListFut =
                CompletableFuture.supplyAsync(() -> historyDao.getUserHistories(userId, null, 10), EXECUTOR);

        User user = userFut.get();
        if (user == null) {
            return sendErrorView(ctx, StatusCode.NOT_FOUND_CODE,"Couldn't find a user for the provided user id.");
        }

        int rank = remoteDict.getLeaderboardRank(user.getId());
        user.setRank(rank);
        List<History> historyList = historyListFut.get();

        user.sanitize();
        historyList.forEach(History::sanitize);

        Template template = templates.getProfileTemplate();
        String resp = template.apply(new ProfileView(user, historyList));

//        ctx.setResponseHeader("Cache-Control", "max-age=60, must-revalidate");
        return resp;
    }

    @Data
    @AllArgsConstructor
    public static class SearchView {
        public String searchText;
        public List<User> userList;
        public Pagination pages;
    }

    public String searchPlayers(Context ctx) throws IOException {
        Templates templates = state.getTemplates();
        UserDao userDao = state.getUserDao();

        try {
            int page = ctx.query("page").toOptional().map(Integer::parseUnsignedInt).orElse(1);
            String name = ctx.query("username").toOptional().orElse("");

            Template template = templates.getSearchTemplate();
            if (name.isEmpty()) {
                return template.apply(new SearchView(name, List.of(), Pagination.ofUnlimited("?", page)));
            }

            List<User> userList = userDao.searchByName(name, page, 20);
            userList.forEach(User::sanitize);

            Pagination pagination = Pagination.ofUnlimited(String.format("?username=%s&", name), page);
            String resp = template.apply(new SearchView(name, userList, pagination));

//            ctx.setResponseHeader("Cache-Control", "max-age=3600, must-revalidate");
            return resp;
        } catch (NumberFormatException ex) {
            return sendErrorView(ctx, StatusCode.BAD_REQUEST_CODE, "Invalid param 'page': must be a positive integer.");
        }
    }

    @Data
    @AllArgsConstructor
    public static class ReplayView {
        public String initialBoard;
        public History history;
    }

    public String getGameHistories(Context ctx) throws IOException {
        Templates templates = state.getTemplates();
        HistoryDao historyDao = state.getHistoryDao();

        Value historyIdSlug = ctx.path("id");
        if (historyIdSlug.isMissing()) {
            return sendErrorView(ctx, StatusCode.BAD_REQUEST_CODE, "Invalid param 'id': must contain id within slug.");
        }
        long historyId = Long.parseUnsignedLong(historyIdSlug.toString());

        History history = historyDao.getHistory(historyId);
        history.sanitize();

        Template template = templates.getGameStateoryTemplate();
        String resp = template.apply(new ReplayView(initialBoardJson, history));

//        ctx.setResponseHeader("Cache-Control", "max-age=86400, must-revalidate"); // this is never updated, we can cache aggressively
        return resp;
    }

    @Data
    @AllArgsConstructor
    public static class ChallengeView {
        public List<Challenge> challengeList;
        public boolean areSent;
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
        Player player = remoteDict.getSession(session.getSessionId());
        if (player == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, "Session has expired, please login again");
        }

        List<Challenge> challengeList;
        boolean areSent;

        switch (participants) {
            case "received": {
                challengeList = challengeDao.getByParticipant(null, player.getId());
                areSent = false;
                break;
            }
            case "sent": {
                challengeList = challengeDao.getByParticipant(player.getId(), null);
                areSent = true;
                break;
            }
            default: {
                int code = StatusCode.BAD_REQUEST_CODE;
                ctx.setResponseCode(code);
                return templates.getErrorTemplate().apply(new ErrorView(code, "Invalid value for participants, must be 'sent' or 'received'."));
            }
        }

        EXECUTOR.execute(() -> challengeDao.deleteExpired(session.getPlayerId()));

        Template template = templates.getChallengesTemplate();
        String resp = template.apply(new ChallengeView(challengeList, areSent));

//        ctx.setResponseHeader("Cache-Control", "max-age=60, must-revalidate");
        return resp;
    }
}
