package web;

import dao.ChallengeDao;
import dao.UserDao;
import services.GameService;
import services.RemoteDict;
import io.jooby.*;
import io.jooby.exception.StatusCodeException;
import models.Challenge;
import models.Player;
import org.jsoup.Jsoup;

import static utils.Globals.*;
import static dao.UserDao.*;
import static web.SessionService.*;

public class FormRouter extends Jooby {
    private final State state;

    public FormRouter(State state) {
        this.state = state;

        setWorker(EXECUTOR);

        post("/forms/register", this::register);
        post("/forms/login", this::login);
        post("/forms/update-password", this::updatePassword);
        post("/forms/update-user", this::updateUser);
        post("/forms/update-session", this::updateSession);
        post("/forms/logout", this::logout);
        post("/forms/create-game", this::createGame);
        post("/forms/update-challenge", this::updateChallenge);
    }

    private static void validatePassword(String passwordStr, String dupPasswordStr) {
        if (passwordStr.length() < 10 || passwordStr.length() > 100) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, "Password should be between 10 and 100 characters");
        }
        if (!passwordStr.equals(dupPasswordStr)) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, "Password and retyped password must be equal");
        }
    }

    public String register(Context ctx) {
        SessionService sessionService = state.getSessionService();
        UserDao userDao = state.getUserDao();
        RemoteDict remoteDict = state.getRemoteDict();

        Formdata form = ctx.form();
        String username = form.get("username").value();
        String password = form.get("password").value();
        String dupPassword = form.get("duplicate-password").value();

        if (username.length() < 5 || username.length() > 20) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, "Field username should be between 5 and 20 characters");
        }
        if (!Jsoup.isValid(username, HTML_SAFELIST)) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, "Field username cannot contain invalid or unsafe characters");
        }
        validatePassword(password, dupPassword);

        try {
            UserInst inst = userDao.insert(username, password);

            String sessionId = sessionService.createId();
            Cookie cookie = sessionService.createCookie(sessionId, inst.getNewId(), inst.getUsername(), inst.getCountry());
            ctx.setResponseCookie(cookie);

            Player player = new Player(inst.getNewId(), inst.getUsername());
            remoteDict.setSession(sessionId, player, cookie.getMaxAge());

            LOGGER.info("Registered a new player={}", player);

            ctx.setResponseHeader("Content-Type", "text/plain");
            return "Signed up successfully!";
        } catch (UserDao.TakenUsernameException ex) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, "Username is already taken, choose another");
        }
    }

    public String login(Context ctx) {
        SessionService sessionService = state.getSessionService();
        UserDao userDao = state.getUserDao();
        RemoteDict remoteDict = state.getRemoteDict();

        Formdata form = ctx.form();
        String username = form.get("username").value();
        String password = form.get("password").value();

        VerifiedPlayer verifiedPlayer = userDao.verify(username, password);
        if (verifiedPlayer == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, "Login credentials are invalid");
        }

        String sessionId = sessionService.createId();
        Cookie cookie = sessionService.createCookie(sessionId, verifiedPlayer.getId(), verifiedPlayer.getUsername(), verifiedPlayer.getCountry());
        ctx.setResponseCookie(cookie);
        remoteDict.setSession(sessionId, new Player(verifiedPlayer.getId(), verifiedPlayer.getUsername()), cookie.getMaxAge());

        LOGGER.info("Player has logged in {}", verifiedPlayer);

        ctx.setResponseHeader("Content-Type", "text/plain");
        ctx.setResponseHeader("Hx-Redirect", "/");
        return "Logged in successfully!";
    }

    public String updatePassword(Context ctx) {
        SessionService sessionService = state.getSessionService();
        UserDao userDao = state.getUserDao();

        Formdata form = ctx.form();
        String password = form.get("password").value();
        String newPassword = form.get("new-password").value();
        String dupPassword = form.get("duplicate-new-password").value();

        validatePassword(password, dupPassword);

        SessionService.SessionValue session = sessionService.parseSession(ctx);
        if (session == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, "Cannot update password when you are not logged in");
        }
        VerifiedPlayer player = userDao.verify(session.getUsername(), password);
        if (player == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, "Login credentials are invalid");
        }

        userDao.updatePassword(player.getId(), newPassword);
        LOGGER.info("Updated data of user={}", player);

        ctx.setResponseHeader("Content-Type", "text/plain");
        return "Updated successfully!";
    }

    public String updateUser(Context ctx) {
        UserDao userDao = state.getUserDao();
        SessionService sessionService = state.getSessionService();
        RemoteDict remoteDict = state.getRemoteDict();

        Formdata form = ctx.form();
        String newUsername = form.get("new-username").valueOrNull();
        String newCountry = form.get("new-country").valueOrNull();
        String newBio = form.get("new-bio").valueOrNull();

        SessionValue session = sessionService.parseSession(ctx);
        if (session == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, "Cannot update user when you are not logged in");
        }
        Player player = remoteDict.getSession(session.getSessionId());
        if (player == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, "Session has expired, please login again");
        }

        userDao.updateUser(player.getId(), newUsername, newCountry, newBio);
        LOGGER.info("Updated data of user={}", session.getPlayerId());

        ctx.setResponseHeader("Content-Type", "text/plain");
        return "Updated successfully!";
    }

    public String updateSession(Context ctx) {
        SessionService sessionService = state.getSessionService();
        RemoteDict remoteDict = state.getRemoteDict();

        SessionValue session = sessionService.parseSession(ctx);
        if (session != null) {
            Cookie cookie = sessionService.createCookie(session.getSessionId(), session.getPlayerId(), session.getUsername(), session.getCountry());
            ctx.setResponseCookie(cookie);
            remoteDict.updateSessionEx(session.getSessionId(), cookie.getMaxAge());

            LOGGER.info("Refreshed session for user={}", session.getPlayerId());
        }

        ctx.setResponseHeader("Content-Type", "text/plain");
        return "Updated session successfully!";
    }

    public String logout(Context ctx) {
        SessionService sessionService = state.getSessionService();
        RemoteDict remoteDict = state.getRemoteDict();

        SessionValue session = sessionService.parseSession(ctx);
        remoteDict.deleteSession(session.getSessionId());
        ctx.setResponseCookie(sessionService.createEmptyCookie());

        LOGGER.info("Logged out user={}", session.getPlayerId());

        ctx.setResponseHeader("Content-Type", "text/plain");
        return "Logged out successfully!";
    }

    public String createGame(Context ctx) {
        GameService gameService = state.getGameService();
        String colorParam = ctx.query("color").toOptional().orElse(null);

        Boolean isFirstWhite = null;
        if (colorParam != null && colorParam.equals("white")) {
            isFirstWhite = true;
        } else if (colorParam != null && colorParam.equals("black")) {
            isFirstWhite = false;
        }

        return gameService.create(isFirstWhite);
    }

    public boolean handleUpdateAction(String callerId, String challengeeId, String challengerId, String action) {
        ChallengeDao challengeDao = state.getChallengeDao();

        switch (action) {
        case "DELETE":
            LOGGER.info("Handling DELETE challenge=[challengee={}, challenger={}] from caller={}", challengeeId, challengerId, callerId);
            if (callerId.equals(challengerId)) {
                challengeDao.deleteChallenge(challengeeId, challengerId);
                return false;
            } else {
                throw new StatusCodeException(StatusCode.UNAUTHORIZED, "You must be the challenger to delete a challenge");
            }
        case "ACCEPT":
            LOGGER.info("Handling ACCEPT challenge=[challengee={}, challenger={}] from caller={}", challengeeId, challengerId, callerId);
            if (callerId.equals(challengeeId)) {
                return challengeDao.updateStatus(challengeeId, challengerId, Challenge.Status.ACCEPTED);
            } else {
                throw new StatusCodeException(StatusCode.UNAUTHORIZED, "You must be the challengee to accept a challenge");
            }
        case "REJECT":
            LOGGER.info("Handling REJECT challenge=[challengee={}, challenger={}] from caller={}", challengeeId, challengerId, callerId);
            if (callerId.equals(challengeeId)) {
                challengeDao.updateStatus(challengeeId, challengerId, Challenge.Status.REJECTED);
                return false;
            } else {
                throw new StatusCodeException(StatusCode.UNAUTHORIZED, "You must be the challengee to reject a challenge");
            }
        default:
            throw new StatusCodeException(StatusCode.BAD_REQUEST, String.format("Invalid challenge action code=%s", action));
        }
    }

    public String updateChallenge(Context ctx) {
        GameService gameService = state.getGameService();
        SessionService sessionService = state.getSessionService();
        RemoteDict remoteDict = state.getRemoteDict();

        Formdata form = ctx.form();
        String challengeeId = form.get("challengeeId").value();
        String challengerId = form.get("challengerId").value();
        String action = form.get("action").value();

        SessionService.SessionValue session = sessionService.parseSession(ctx);
        if (session == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, "Cannot update user when you are not logged in");
        }
        Player player = remoteDict.getSession(session.getSessionId());
        if (player == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, "Session has expired, please login again");
        }

        String callerId = player.getId();

        boolean accept = handleUpdateAction(callerId, challengeeId, challengerId, action);
        if (accept) {
            String id = gameService.create(null);

            ctx.setResponseCode(StatusCode.CREATED);
            ctx.setResponseHeader("Content-Type", "text/plain");
            return id;
        } else {
            ctx.setResponseCode(StatusCode.OK);
            ctx.setResponseHeader("Content-Type", "text/plain");
            return "Successfully updated challenge!";
        }
    }
}
