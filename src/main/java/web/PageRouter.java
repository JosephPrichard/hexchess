package web;

import domain.ChessBoard;
import io.jooby.Jooby;
import io.jooby.MediaType;
import io.jooby.Route;
import io.jooby.StatusCode;
import lombok.AllArgsConstructor;
import lombok.Data;
import models.History;
import models.Pagination;
import models.User;

import java.io.IOException;
import java.util.List;
import java.util.concurrent.CompletableFuture;

import static utils.Globals.*;

public class PageRouter extends Jooby {

    @Data
    @AllArgsConstructor
    public static class LeaderboardView {
        List<User> userList;
        Pagination pages;
    }

    @Data
    @AllArgsConstructor
    public static class ProfileView {
        User user;
        List<History> historyList;
    }

    @Data
    @AllArgsConstructor
    public static class SearchView {
        String searchText;
        List<User> userList;
        Pagination pages;
    }

    @Data
    @AllArgsConstructor
    public static class ReplayView {
        String initialBoard;
        History history;
    }

    @Data
    @AllArgsConstructor
    public static class ErrorView {
        int code;
        String message;
    }

    private final String initialBoardJson;
    private final String loginHtml;
    private final String registerHtml;

    public PageRouter(State state) {
        var userDao = state.getUserDao();
        var historyDao = state.getHistoryDao();
        var gameService = state.getGameService();
        var remoteDict = state.getRemoteDict();
        var sessionService = state.getSessionService();
        var templates = state.getTemplates();

        try {
            var loginTemplate = templates.getLoginTemplate();
            var registerTemplate = templates.getRegisterTemplate();

            initialBoardJson = JSON_MAPPER.writeValueAsString(ChessBoard.initial());
            loginHtml = loginTemplate != null ? loginTemplate.apply(null) : "";
            registerHtml = registerTemplate != null ? registerTemplate.apply(null) : "";
        } catch (IOException ex) {
            LOGGER.error("Failed during page document initialization", ex);
            throw new RuntimeException(ex);
        }

        setWorker(EXECUTOR);

        use(next -> ctx -> {
            ctx.setResponseType(MediaType.HTML);
            return next.apply(ctx);
        });

        get("*", ctx -> {
            var template = templates.getErrorTemplate();
            var message = """
                Sorry, the page you are looking for does not exist.
                You might have followed a broken link or entered a URL that doesn't exist on this site.
                """;
            var code = StatusCode.NOT_FOUND_CODE;
            ctx.setResponseCode(code);
            return template.apply(new ErrorView(code, message));
        });

        Route.Handler indexHandler = (ctx) -> {
//            var gamesResult = gameService.getGames(null);
            var template = templates.getIndexTemplate();
            return template.apply(null);
        };

        get("/", indexHandler);

        get("/index", indexHandler);

        get("/play", indexHandler);

        get("/login", ctx -> loginHtml);

        get("/register", ctx -> registerHtml);

        // this route is un-cacheable due to using cookies
        get("/settings", ctx -> {
            var cookieStr = ctx.header("Cookie").valueOrNull();

            var session = sessionService.getSession(cookieStr);
            if (session == null) {
                var code = StatusCode.UNAUTHORIZED_CODE;
                ctx.setResponseCode(code);
                var template = templates.getErrorTemplate();
                return template.apply(new ErrorView(code, "You must be logged in to access this page."));
            }

            var user = userDao.getById(session.getPlayerId());

            var template = templates.getPreferencesTemplates();
            return template.apply(user);
        });

        get("/leaderboard", ctx -> {
            try {
                int page = ctx.query("page").toOptional().map(Integer::parseUnsignedInt).orElse(1);

                var userListFut = CompletableFuture.supplyAsync(() -> userDao.getLeaderboard(page, 25), EXECUTOR);
                var totalPagesFut = CompletableFuture.supplyAsync(() -> userDao.countPages(25), EXECUTOR);
                var userList = userListFut.get();
                var totalPages = totalPagesFut.get();

                userList.forEach(User::sanitize);

                var template = templates.getLeaderboardTemplate();
                return template.apply(new LeaderboardView(userList, Pagination.withTotal("?", page, totalPages)));
            } catch (NumberFormatException ex) {
                var code = StatusCode.BAD_REQUEST_CODE;
                ctx.setResponseCode(code);
                var template = templates.getErrorTemplate();
                return template.apply(new ErrorView(code, "Invalid param 'page': must be a positive integer."));
            }
        });

//        get("/leaderboard", ctx -> {
//            try {
//                int page = ctx.query("page").toOptional().map(Integer::parseUnsignedInt).orElse(1);
//
//                var leaderboard = remoteDict.getLeaderboardPage(page, 20);
//                var userList = userDao.getByRanks(leaderboard.getUsers());
//
//                RankedUser.joinRanks(leaderboard.getUsers(), userList);
//                userList.forEach(User::sanitize);
//
//                var template = templates.getLeaderboardTemplate();
//                return template.apply(new LeaderboardView(userList, Pagination.withTotal("?", page, leaderboard.getPageCount())));
//            } catch (NumberFormatException ex) {
//                var code = StatusCode.BAD_REQUEST_CODE;
//                ctx.setResponseCode(code);
//                var template = templates.getErrorTemplate();
//                return template.apply(new ErrorView(code, "Invalid param 'page': must be a positive integer."));
//            }
//        });

        get("/players/{id}", ctx -> {
            var userIdSlug = ctx.path("id");
            if (userIdSlug.isMissing()) {
                var code = StatusCode.BAD_REQUEST_CODE;
                ctx.setResponseCode(code);
                var template = templates.getErrorTemplate();
                return template.apply(new ErrorView(code, "Invalid param 'id': must contain id within slug."));
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
        });

//        get("/players/{id}", ctx -> {
//            var userIdSlug = ctx.path("id");
//            if (userIdSlug.isMissing()) {
//                var code = StatusCode.BAD_REQUEST_CODE;
//                ctx.setResponseCode(code);
//                var template = templates.getErrorTemplate();
//                return template.apply(new ErrorView(code, "Invalid param 'id': must contain id within slug."));
//            }
//            var userId = userIdSlug.toString();
//
//            var userFut = CompletableFuture.supplyAsync(() -> userDao.getById(userId), EXECUTOR);
//            var historyListFut = CompletableFuture.supplyAsync(() -> historyDao.getUserHistories(userId, null, 10), EXECUTOR);
//
//            var user = userFut.get();
//            var rank = remoteDict.getLeaderboardRank(user.getId());
//            user.setRank(rank);
//            var historyList = historyListFut.get();
//
//            user.sanitize();
//            historyList.forEach(History::sanitize);
//
//            var template = templates.getProfileTemplate();
//            return template.apply(new ProfileView(user, historyList));
//        });

        get("/players/search", ctx -> {
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
                var template = templates.getErrorTemplate();
                return template.apply(new ErrorView(code, "Invalid param 'page': must be a positive integer."));
            }
        });

        get("/games/histories/{id}", ctx -> {
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
        });
    }
}
