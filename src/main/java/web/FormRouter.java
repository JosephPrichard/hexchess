package web;

import io.jooby.Context;
import io.jooby.Jooby;
import io.jooby.StatusCode;
import io.jooby.exception.StatusCodeException;
import models.Player;
import org.jsoup.Jsoup;
import dao.UserDao;

import static utils.Globals.*;

public class FormRouter extends Jooby {
    private final State state;

    public FormRouter(State state) {
        this.state = state;

        setWorker(EXECUTOR);

        post("/forms/signup", this::signup);
        post("/forms/login", this::login);
        post("/forms/update-password", this::updatePassword);
        post("/forms/update-user", this::updateUser);
        post("/forms/update-session", this::updateSession);
        post("/forms/logout", this::logout);
        post("/forms/create-game", this::createGame);
    }

    public String signup(Context ctx) {
        var sessionService = state.getSessionService();
        var userDao = state.getUserDao();
        var remoteDict = state.getRemoteDict();

        var form = ctx.form();
        var username = form.get("username");
        var password = form.get("password");
        var dupPassword = form.get("duplicate-password");

        if (username.isMissing() || password.isMissing()) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, "Form data is invalid");
        }

        var usernameStr = username.toString();
        var passwordStr = password.toString();
        var dupPasswordStr = dupPassword.toString();

        if (usernameStr.length() < 5 || usernameStr.length() > 20) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, "Username should be between 5 and 20 characters");
        }
        if (!Jsoup.isValid(usernameStr, HTML_SAFELIST)) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, "Username cannot contain invalid or unsafe characters");
        }
        validatePassword(passwordStr, dupPasswordStr);

        try {
            var inst = userDao.insert(usernameStr, passwordStr);

            var sessionId = sessionService.createId();
            var cookie = sessionService.createCookie(sessionId, inst.getNewId(), inst.getUsername(), inst.getCountry());
            ctx.setResponseCookie(cookie);

            var player = new Player(inst.getNewId(), inst.getUsername());
            remoteDict.setSession(sessionId, player, cookie.getMaxAge());

            LOGGER.info("Registered a new player={}", player);

            ctx.setResponseHeader("Content-Type", "text/plain");
            return "Signed up successfully!";
        } catch (UserDao.TakenUsernameException ex) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, "Username is already taken, choose another");
        }
    }

    public String login(Context ctx) {
        var sessionService = state.getSessionService();
        var userDao = state.getUserDao();
        var remoteDict = state.getRemoteDict();

        var form = ctx.form();
        var username = form.get("username");
        var password = form.get("password");
        if (username.isMissing() || password.isMissing()) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, "Form data is invalid");
        }

        var verifiedPlayer = userDao.verify(username.toString(), password.toString());
        if (verifiedPlayer == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, "Login credentials are invalid");
        }

        var sessionId = sessionService.createId();
        var cookie = sessionService.createCookie(sessionId, verifiedPlayer.getId(), verifiedPlayer.getUsername(), verifiedPlayer.getCountry());
        ctx.setResponseCookie(cookie);
        remoteDict.setSession(sessionId, new Player(verifiedPlayer.getId(), verifiedPlayer.getUsername()), cookie.getMaxAge());

        LOGGER.info("Player has logged in {}", verifiedPlayer);

        ctx.setResponseHeader("Content-Type", "text/plain");
        return "Logged in successfully!";
    }

    public String updatePassword(Context ctx) {
        var sessionService = state.getSessionService();
        var userDao = state.getUserDao();

        var form = ctx.form();
        var passwordStr = form.get("password").toString();
        var newPasswordStr = form.get("new-password").toString();
        var dupPasswordStr = form.get("duplicate-new-password").toString();

        validatePassword(passwordStr, dupPasswordStr);

        var session = sessionService.getSession(ctx.header("Cookie").valueOrNull());
        if (session == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, "Cannot update password when you are not logged in");
        }

        var player = userDao.verify(session.getUsername(), passwordStr);
        if (player == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, "Login credentials are invalid");
        }
        userDao.updatePassword(player.getId(), newPasswordStr);

        LOGGER.info("Updated data of user={}", player);

        ctx.setResponseHeader("Content-Type", "text/plain");
        return "Updated successfully!";
    }

    public String updateUser(Context ctx) {
        var sessionService = state.getSessionService();
        var userDao = state.getUserDao();

        var form = ctx.form();
        var newUsernameStr = form.get("new-username").valueOrNull();
        var newCountryStr = form.get("new-country").valueOrNull();
        var newBioStr = form.get("new-bio").valueOrNull();

        var session = sessionService.getSession(ctx.header("Cookie").valueOrNull());
        if (session == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, "Cannot update user when you are not logged in");
        }

        userDao.updateUser(session.getPlayerId(), newUsernameStr, newCountryStr, newBioStr);
        LOGGER.info("Updated data of user={}", session.getPlayerId());

        ctx.setResponseHeader("Content-Type", "text/plain");
        return "Updated successfully!";
    }

    public String updateSession(Context ctx) {
        var sessionService = state.getSessionService();
        var remoteDict = state.getRemoteDict();

        var session = sessionService.getSession(ctx.header("Cookie").valueOrNull());
        if (session != null) {
            var cookie = sessionService.createCookie(session.getSessionId(), session.getPlayerId(), session.getUsername(), session.getCountry());
            ctx.setResponseCookie(cookie);
            remoteDict.updateSessionEx(session.getSessionId(), cookie.getMaxAge());

            LOGGER.info("Refreshed session for user={}", session.getPlayerId());
        }

        ctx.setResponseHeader("Content-Type", "text/plain");
        return "Updated session successfully!";
    }

    public String logout(Context ctx) {
        var sessionService = state.getSessionService();
        var remoteDict = state.getRemoteDict();

        var session = sessionService.getSession(ctx.header("Cookie").valueOrNull());
        remoteDict.deleteSession(session.getSessionId());
        ctx.setResponseCookie(sessionService.createEmptyCookie());

        LOGGER.info("Logged out user={}", session.getPlayerId());

        ctx.setResponseHeader("Content-Type", "text/plain");
        return "Logged out successfully!";
    }

    public String createGame(Context ctx) {
        var gameService = state.getGameService();
        var colorParam = ctx.query("color").toOptional().orElse(null);

        Boolean isFirstWhite = null;
        if (colorParam != null && colorParam.equals("white")) {
            isFirstWhite = true;
        } else if (colorParam != null && colorParam.equals("black")) {
            isFirstWhite = false;
        }

        return gameService.create(isFirstWhite);
    }

    private static void validatePassword(String passwordStr, String dupPasswordStr) {
        if (passwordStr.length() < 10 || passwordStr.length() > 100) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, "Password should be between 10 and 100 characters");
        }
        if (!passwordStr.equals(dupPasswordStr)) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, "Password and retyped password must be equal");
        }
    }
}
