package web;

import dao.ChallengeDao;
import dao.UserDao;
import services.GameService;
import services.RemoteDict;
import io.jooby.*;
import io.jooby.exception.StatusCodeException;
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
        post("/forms/create-challenge", this::createChallenge);
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
            remoteDict.incrLeaderboardUser(inst.getNewId(), inst.getElo());

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

        VerifiedUser verifiedUser = userDao.verify(username, password);
        if (verifiedUser == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, "Login credentials are invalid");
        }

        String sessionId = sessionService.createId();
        Cookie cookie = sessionService.createCookie(sessionId, verifiedUser.getId(), verifiedUser.getUsername(), verifiedUser.getCountry());
        ctx.setResponseCookie(cookie);
        remoteDict.setSession(sessionId, new Player(verifiedUser.getId(), verifiedUser.getUsername()), cookie.getMaxAge());

        LOGGER.info("Player has logged in {}", verifiedUser);

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
        VerifiedUser player = userDao.verify(session.getUsername(), password);
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
            ctx.setResponseCookie(sessionService.createEmptyCookie());
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, "Session has expired, please login again");
        }

        VerifiedUser verifiedUser = userDao.updateUser(player.getId(), newUsername, newCountry, newBio);
        if (verifiedUser != null) {
            LOGGER.info("Updated data of user={}", session.getPlayerId());
            Cookie cookie = sessionService.createCookie(session.getSessionId(), verifiedUser.getId(), verifiedUser.getUsername(), verifiedUser.getCountry());
            ctx.setResponseCookie(cookie);
        }

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

    public String updateChallenge(Context ctx) {
        GameService gameService = state.getGameService();
        SessionService sessionService = state.getSessionService();
        RemoteDict remoteDict = state.getRemoteDict();
        ChallengeDao challengeDao = state.getChallengeDao();

        Formdata form = ctx.form();
        String challengeeId = form.get("challengeeId").value();
        String challengerId = form.get("challengerId").value();
        String action = form.get("action").value().toUpperCase();

        SessionService.SessionValue session = sessionService.parseSession(ctx);
        if (session == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, "Cannot update user when you are not logged in");
        }
        Player player = remoteDict.getSession(session.getSessionId());
        if (player == null) {
            ctx.setResponseCookie(sessionService.createEmptyCookie());
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, "Session has expired, please login again");
        }

        String callerId = player.getId();

        switch (action) {
        case "ACCEPT":
            if (callerId.equals(challengeeId)) {
                int count = challengeDao.delete(challengerId, challengeeId);
                if (count == 0) {
                    throw new StatusCodeException(StatusCode.NOT_FOUND, "Failed to accept the challenge, it does not exist anymore");
                }

                String id = gameService.create(null);

                ctx.setResponseCode(StatusCode.CREATED);
                ctx.setResponseHeader("Content-Type", "text/plain");
                return id;
            } else {
                throw new StatusCodeException(StatusCode.UNAUTHORIZED, "You must be the challengee to accept a challenge");
            }
        case "DELETE":
            if (callerId.equals(challengerId)) {
                int count = challengeDao.delete(challengerId, challengeeId);
                if (count == 0) {
                    throw new StatusCodeException(StatusCode.NOT_FOUND, "Failed to delete the challenge, it does not exist anymore");
                }

                ctx.setResponseCode(StatusCode.OK);
                ctx.setResponseHeader("Content-Type", "text/plain");
                return "Successfully updated challenge";
            } else {
                throw new StatusCodeException(StatusCode.UNAUTHORIZED, "You must be the challenger to delete a challenge");
            }
        case "REJECT":
            if (callerId.equals(challengeeId)) {
                int count = challengeDao.delete(challengerId, challengeeId);
                if (count == 0) {
                    throw new StatusCodeException(StatusCode.NOT_FOUND, "Failed to reject the challenge, it does not exist anymore");
                }

                ctx.setResponseCode(StatusCode.OK);
                ctx.setResponseHeader("Content-Type", "text/plain");
                return "Successfully updated challenge";
            } else {
                throw new StatusCodeException(StatusCode.UNAUTHORIZED, "You must be the challengee to reject a challenge");
            }
        default:
            throw new StatusCodeException(StatusCode.BAD_REQUEST, String.format("Invalid challenge action code=%s", action));
        }
    }

    public String createChallenge(Context ctx) {
        SessionService sessionService = state.getSessionService();
        RemoteDict remoteDict = state.getRemoteDict();
        ChallengeDao challengeDao = state.getChallengeDao();

        Formdata form = ctx.form();
        String challengeeId = form.get("challengeeId").value();

        SessionService.SessionValue session = sessionService.parseSession(ctx);
        if (session == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, "Cannot update user when you are not logged in");
        }
        Player player = remoteDict.getSession(session.getSessionId());
        if (player == null) {
            ctx.setResponseCookie(sessionService.createEmptyCookie());
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, "Session has expired, please login again");
        }

        try {
            challengeDao.insert(challengeeId, player.getId());
        } catch (ChallengeDao.ParticipantException ex) {
            throw new StatusCodeException(StatusCode.NOT_FOUND, "The challenge is made against a participant that does not exist");
        } catch (ChallengeDao.SelfException ex) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, "You cannot challenge yourself");
        } catch (ChallengeDao.DuplicateException ex) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, "A challenge against this player already exists");
        }

        return "Created a challenge against player";
    }
}
