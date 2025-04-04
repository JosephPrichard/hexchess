package web.routers;

import io.jooby.jackson.JacksonModule;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;
import services.dao.ChallengeDao;
import services.dao.UserDao;
import models.UserEntity;
import services.GameService;
import services.RemoteDict;
import io.jooby.*;
import io.jooby.exception.StatusCodeException;
import models.PlayerEntity;
import org.jsoup.Jsoup;
import web.SessionService;
import web.State;

import static utils.Globals.*;
import static services.dao.UserDao.*;
import static web.SessionService.*;

public class FormRouter extends Jooby {

    private final State state;

    public FormRouter(State state) {
        this.state = state;

        setWorker(EXECUTOR);

        post("/forms/register", this::register);
        post("/forms/login", this::login);
        post("/forms/users/password", this::updatePassword);
        post("/forms/users", this::updateUser);
        post("/forms/session", this::updateSession);
        post("/forms/logout", this::logout);
        post("/forms/game", this::createGame);
        post("/forms/challenges/update", this::updateChallenge);
        post("/forms/challenges/create", this::createChallenge);
    }

    private static void validatePassword(String passwordStr, String dupPasswordStr) {
        if (passwordStr.length() < 10 || passwordStr.length() > 100) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, "Password should be between 10 and 100 characters");
        }
        if (!passwordStr.equals(dupPasswordStr)) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, "Password and retyped password must be equal");
        }
    }

    @Data
    @NoArgsConstructor
    public static class RegisterBody {
        public String username;
        public String password;
        public String confirmPassword;
    }

    public String register(Context ctx) {
        SessionService sessionService = state.getSessionService();
        UserDao userDao = state.getUserDao();
        RemoteDict remoteDict = state.getRemoteDict();

        RegisterBody body = ctx.body(RegisterBody.class);
        String username = body.getUsername();
        String password = body.getPassword();
        String confirmPassword = body.getConfirmPassword();

        if (username.length() < 5 || username.length() > 20) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, "Field username should be between 5 and 20 characters");
        }
        if (!Jsoup.isValid(username, HTML_SAFELIST)) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, "Field username cannot contain invalid or unsafe characters");
        }
        validatePassword(password, confirmPassword);

        try {
            UserEntity user = userDao.insert(username, password);
            remoteDict.incrLeaderboardUser(user.getId(), user.getElo());

            String sessionId = sessionService.createId();
            Cookie cookie = sessionService.createCookie(sessionId, user.getId(), user.getUsername(), user.getCountry());
            ctx.setResponseCookie(cookie);

            PlayerEntity player = new PlayerEntity(user.getId(), user.getUsername());
            remoteDict.setSession(sessionId, player, cookie.getMaxAge());

            LOGGER.info("Registered a new player={}", player);

            ctx.setResponseHeader("Content-Type", "text/plain");
            return "Signed up successfully!";
        } catch (UserDao.TakenUsernameException ex) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, "Username is already taken, choose another");
        }
    }

    @Data
    @NoArgsConstructor
    public static class LoginBody {
        public String username;
        public String password;
    }

    public String login(Context ctx) {
        SessionService sessionService = state.getSessionService();
        UserDao userDao = state.getUserDao();
        RemoteDict remoteDict = state.getRemoteDict();

        LoginBody body = ctx.body(LoginBody.class);
        String username = body.getUsername();
        String password = body.getPassword();

        VerifiedUser verifiedUser = userDao.verify(username, password);
        if (verifiedUser == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, "Login credentials are invalid");
        }

        String sessionId = sessionService.createId();
        Cookie cookie = sessionService.createCookie(sessionId, verifiedUser.getId(), verifiedUser.getUsername(), verifiedUser.getCountry());
        ctx.setResponseCookie(cookie);
        remoteDict.setSession(sessionId, new PlayerEntity(verifiedUser.getId(), verifiedUser.getUsername()), cookie.getMaxAge());

        LOGGER.info("Player has logged in {}", verifiedUser);

        ctx.setResponseHeader("Content-Type", "text/plain");
        ctx.setResponseHeader("Hx-Redirect", "/");
        return "Logged in successfully!";
    }

    @Data
    @NoArgsConstructor
    public static class UpdatePasswordBody {
        public String password;
        public String newPassword;
        public String confirmNewPassword;
    }

    public String updatePassword(Context ctx) {
        SessionService sessionService = state.getSessionService();
        UserDao userDao = state.getUserDao();

        UpdatePasswordBody body = ctx.body(UpdatePasswordBody.class);
        String password = body.getPassword();
        String newPassword = body.getNewPassword();
        String confirmNewPassword = body.getConfirmNewPassword();

        validatePassword(password, confirmNewPassword);

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

    @Data
    @NoArgsConstructor
    public static class UpdateUserBody {
        public String newUsername;
        public String newCountry;
        public String newBio;
    }

    public String updateUser(Context ctx) {
        UserDao userDao = state.getUserDao();
        SessionService sessionService = state.getSessionService();
        RemoteDict remoteDict = state.getRemoteDict();

        UpdateUserBody body = ctx.body(UpdateUserBody.class);
        String newUsername = body.getNewUsername();
        String newCountry = body.getNewCountry();
        String newBio = body.getNewBio();

        SessionValue session = sessionService.parseSession(ctx);
        if (session == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, "Cannot update user when you are not logged in");
        }
        PlayerEntity player = remoteDict.getSession(session.getSessionId());
        if (player == null) {
            ctx.setResponseCookie(sessionService.createEmptyCookie());
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, "Session has expired, please login again");
        }

        VerifiedUser verifiedUser = userDao.updateUser(player.getId(), newUsername, newCountry, newBio);
        if (verifiedUser != null) {
            LOGGER.info("Updated data of user={}", session.getUserId());
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
            Cookie cookie = sessionService.createCookie(session.getSessionId(), session.getUserId(), session.getUsername(), session.getCountry());
            ctx.setResponseCookie(cookie);
            remoteDict.updateSessionEx(session.getSessionId(), cookie.getMaxAge());

            LOGGER.info("Refreshed session for user={}", session.getUserId());
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

        LOGGER.info("Logged out user={}", session.getUserId());

        ctx.setResponseHeader("Content-Type", "text/plain");
        return "Logged out successfully!";
    }

    @Data
    @NoArgsConstructor
    public static class CreateGameBody {
        public String color;
    }

    public String createGame(Context ctx) {
        GameService gameService = state.getGameService();

        CreateGameBody body = ctx.body(CreateGameBody.class);
        String colorParam = body.getColor();

        Boolean isFirstWhite = null;
        if (colorParam != null && colorParam.equals("white")) {
            isFirstWhite = true;
        } else if (colorParam != null && colorParam.equals("black")) {
            isFirstWhite = false;
        }

        return gameService.create(isFirstWhite);
    }

    @Data
    @NoArgsConstructor
    public static class UpdateChallengeBody {
        public long challengeeId;
        public long challengerId;
        public String action;
    }

    public String updateChallenge(Context ctx) {
        GameService gameService = state.getGameService();
        SessionService sessionService = state.getSessionService();
        RemoteDict remoteDict = state.getRemoteDict();
        ChallengeDao challengeDao = state.getChallengeDao();

        UpdateChallengeBody body = ctx.body(UpdateChallengeBody.class);
        long challengeeId = body.getChallengeeId();
        long challengerId = body.getChallengerId();
        String action = body.getAction();

        SessionService.SessionValue session = sessionService.parseSession(ctx);
        if (session == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, "Cannot update user when you are not logged in");
        }
        PlayerEntity player = remoteDict.getSession(session.getSessionId());
        if (player == null) {
            ctx.setResponseCookie(sessionService.createEmptyCookie());
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, "Session has expired, please login again");
        }

        long callerId = player.getId();

        switch (action) {
        case "ACCEPT":
            if (callerId == challengeeId) {
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
            if (callerId == challengerId) {
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
            if (callerId == challengeeId) {
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

    @Data
    @NoArgsConstructor
    public static class CreateChallengeBody {
        public long challengeeId;
    }

    public String createChallenge(Context ctx) {
        SessionService sessionService = state.getSessionService();
        RemoteDict remoteDict = state.getRemoteDict();
        ChallengeDao challengeDao = state.getChallengeDao();

        CreateChallengeBody body = ctx.body(CreateChallengeBody.class);
        long challengeeId = body.getChallengeeId();

        SessionService.SessionValue session = sessionService.parseSession(ctx);
        if (session == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, "Cannot update user when you are not logged in");
        }
        PlayerEntity player = remoteDict.getSession(session.getSessionId());
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
