package web.controllers;

import lombok.Data;
import lombok.NoArgsConstructor;
import daos.ChallengeDao;
import daos.UserDao;
import models.UserEntity;
import services.GameService;
import services.RemoteDict;
import io.jooby.*;
import io.jooby.exception.StatusCodeException;
import models.PlayerEntity;
import org.jsoup.Jsoup;
import services.SessionService;
import web.State;

import static utils.Globals.*;
import static daos.UserDao.*;
import static services.SessionService.*;

public class FormController extends Jooby {

    private static final String ERROR_INVALID_PASSWORD = "ERROR_PASSWORD_LENGTH";
    private static final String ERROR_CONFIRM_PASSWORD = "ERROR_CONFIRM_PASSWORD";
    private static final String ERROR_INVALID_USERNAME = "ERROR_USERNAME_LENGTH";
    private static final String ERROR_UNSAFE_USERNAME = "ERROR_UNSAFE_USERNAME";
    private static final String ERROR_DUPLICATE_USERNAME = "ERROR_DUPLICATE_USERNAME";
    private static final String ERROR_INVALID_LOGIN = "ERROR_INVALID_LOGIN";
    private static final String ERROR_REQUIRED_LOGIN = "ERROR_REQUIRED_LOGIN";
    private static final String ERROR_SESSION_EXPIRED = "ERROR_SESSION_EXPIRED";
    private static final String ERROR_NOT_FOUND_CHALLENGE = "ERROR_NOT_FOUND_CHALLENGE";
    private static final String ERROR_INVALID_CHALLENGE_ACTION = "ERROR_INVALID_CHALLENGE_ACTION";
    private static final String ERROR_SELF_CHALLENGE = "ERROR_SELF_CHALLENGE";
    private static final String ERROR_DUPLICATE_CHALLENGE = "ERROR_DUPLICATE_CHALLENGE";
    private static final String ERROR_UPDATE_CHALLENGE = "ERROR_UPDATE_CHALLENGE";
    private static final String SUCCESS_LOGIN = "SUCCESS_LOGIN";
    private static final String SUCCESS_REGISTER = "SUCCESS_REGISTER";
    private static final String SUCCESS_UPDATE_PASSWORD = "SUCCESS_UPDATE_PASSWORD";
    private static final String SUCCESS_UPDATE_USER = "SUCCESS_UPDATE_USER";
    private static final String SUCCESS_CREATE_CHALLENGE = "SUCCESS_CREATE_CHALLENGE";
    private static final String SUCCESS_UPDATE_CHALLENGE = "SUCCESS_UPDATE_CHALLENGE";
    private static final String SUCCESS_GENERIC = "SUCCESS";

    private final State state;

    public FormController(State state) {
        this.state = state;

        setWorker(EXECUTOR);

        error((ctx, cause, statusCode) -> {
            ctx.setResponseCode(statusCode);
            if (statusCode.value() == 500) {
                LOGGER.error("Error: {} ", statusCode, cause);
                ctx.send("Internal Server Error");
            } else {
                String message = "Error: " + statusCode + ", " + cause.getMessage();
                LOGGER.error(message);
                ctx.send(cause.getMessage());
            }
        });

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

    private static void validatePassword(String password, String confirmPassword) {
        if (password.length() < 10 || password.length() > 100) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, ERROR_INVALID_PASSWORD);
        }
        if (!password.equals(confirmPassword)) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, ERROR_CONFIRM_PASSWORD);
        }
    }

    @Data
    @NoArgsConstructor
    static class RegisterBody {
        String username;
        String password;
        String confirmPassword;
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
            throw new StatusCodeException(StatusCode.BAD_REQUEST, ERROR_INVALID_USERNAME);
        }
        if (!Jsoup.isValid(username, HTML_SAFELIST)) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, ERROR_UNSAFE_USERNAME);
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

            return SUCCESS_REGISTER;
        } catch (UserDao.TakenUsernameException ex) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, ERROR_DUPLICATE_USERNAME);
        }
    }

    @Data
    @NoArgsConstructor
    static class LoginBody {
        String username;
        String password;
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
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, ERROR_INVALID_LOGIN);
        }

        String sessionId = sessionService.createId();
        Cookie cookie = sessionService.createCookie(sessionId, verifiedUser.getId(), verifiedUser.getUsername(), verifiedUser.getCountry());
        ctx.setResponseCookie(cookie);
        remoteDict.setSession(sessionId, new PlayerEntity(verifiedUser.getId(), verifiedUser.getUsername()), cookie.getMaxAge());

        LOGGER.info("Player has logged in {}", verifiedUser);

        return SUCCESS_LOGIN;
    }

    @Data
    @NoArgsConstructor
    static class UpdatePasswordBody {
        String password;
        String newPassword;
        String confirmNewPassword;
    }

    public String updatePassword(Context ctx) {
        SessionService sessionService = state.getSessionService();
        UserDao userDao = state.getUserDao();

        UpdatePasswordBody body = ctx.body(UpdatePasswordBody.class);
        String password = body.getPassword();
        String newPassword = body.getNewPassword();
        String confirmNewPassword = body.getConfirmNewPassword();

        validatePassword(newPassword, confirmNewPassword);

        SessionService.SessionValue session = sessionService.parseSession(ctx);
        if (session == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, ERROR_REQUIRED_LOGIN);
        }
        VerifiedUser player = userDao.verify(session.getUsername(), password);
        if (player == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, ERROR_INVALID_LOGIN);
        }

        userDao.updatePassword(player.getId(), newPassword);

        return SUCCESS_UPDATE_PASSWORD;
    }

    @Data
    @NoArgsConstructor
    static class UpdateUserBody {
        String newUsername;
        String newCountry;
        String newBio;
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
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, ERROR_REQUIRED_LOGIN);
        }
        PlayerEntity player = remoteDict.getSession(session.getSessionId());
        if (player == null) {
            ctx.setResponseCookie(sessionService.createEmptyCookie());
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, ERROR_SESSION_EXPIRED);
        }

        VerifiedUser verifiedUser = userDao.updateUser(player.getId(), newUsername, newCountry, newBio);
        if (verifiedUser != null) {
            Cookie cookie = sessionService.createCookie(session.getSessionId(), verifiedUser.getId(), verifiedUser.getUsername(), verifiedUser.getCountry());
            ctx.setResponseCookie(cookie);
        }

        return SUCCESS_UPDATE_USER;
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

        return SUCCESS_GENERIC;
    }

    public String logout(Context ctx) {
        SessionService sessionService = state.getSessionService();
        RemoteDict remoteDict = state.getRemoteDict();

        SessionValue session = sessionService.parseSession(ctx);
        remoteDict.deleteSession(session.getSessionId());
        ctx.setResponseCookie(sessionService.createEmptyCookie());

        LOGGER.info("Logged out user={}", session.getUserId());

        return SUCCESS_GENERIC;
    }

    @Data
    @NoArgsConstructor
    static class CreateGameBody {
        String color;
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

        ctx.setResponseType(MediaType.TEXT);
        return gameService.create(isFirstWhite);
    }

    @Data
    @NoArgsConstructor
    static class UpdateChallengeBody {
        long challengeeId;
        long challengerId;
        String action;
    }

    public String updateChallenge(Context ctx) {
        GameService gameService = state.getGameService();
        SessionService sessionService = state.getSessionService();
        RemoteDict remoteDict = state.getRemoteDict();
        ChallengeDao challengeDao = state.getChallengeDao();

        UpdateChallengeBody body = ctx.body(UpdateChallengeBody.class);
        long challengeeId = body.getChallengeeId();
        long challengerId = body.getChallengerId();
        String action = body.getAction().toUpperCase();

        SessionService.SessionValue session = sessionService.parseSession(ctx);
        if (session == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, ERROR_REQUIRED_LOGIN);
        }
        PlayerEntity player = remoteDict.getSession(session.getSessionId());
        if (player == null) {
            ctx.setResponseCookie(sessionService.createEmptyCookie());
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, ERROR_SESSION_EXPIRED);
        }

        long callerId = player.getId();

        long targetId = switch (action) {
            case "ACCEPT", "REJECT" -> challengeeId; // an accepter or rejected must be the challengee
            case "DELETE" -> challengerId; // a deleter must be the challenger
            default ->
                throw new StatusCodeException(StatusCode.BAD_REQUEST, ERROR_INVALID_CHALLENGE_ACTION);
        };

        if (callerId == targetId) {
            int count = challengeDao.delete(challengerId, challengeeId);
            if (count == 0) {
                throw new StatusCodeException(StatusCode.NOT_FOUND, ERROR_NOT_FOUND_CHALLENGE);
            }
        } else {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, ERROR_UPDATE_CHALLENGE);
        }

        ctx.setResponseCode(StatusCode.OK);
        return SUCCESS_UPDATE_CHALLENGE;
    }

    @Data
    @NoArgsConstructor
    static class CreateChallengeBody {
        long challengeeId;
    }

    public String createChallenge(Context ctx) {
        SessionService sessionService = state.getSessionService();
        RemoteDict remoteDict = state.getRemoteDict();
        ChallengeDao challengeDao = state.getChallengeDao();

        CreateChallengeBody body = ctx.body(CreateChallengeBody.class);
        long challengeeId = body.getChallengeeId();

        SessionService.SessionValue session = sessionService.parseSession(ctx);
        if (session == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, ERROR_REQUIRED_LOGIN);
        }
        PlayerEntity player = remoteDict.getSession(session.getSessionId());
        if (player == null) {
            ctx.setResponseCookie(sessionService.createEmptyCookie());
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, ERROR_SESSION_EXPIRED);
        }

        try {
            challengeDao.insert(challengeeId, player.getId());
            EXECUTOR.execute(() -> challengeDao.deleteExpired(session.getUserId()));
        } catch (ChallengeDao.ParticipantException ex) {
            throw new StatusCodeException(StatusCode.NOT_FOUND, ERROR_NOT_FOUND_CHALLENGE);
        } catch (ChallengeDao.SelfException ex) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, ERROR_SELF_CHALLENGE);
        } catch (ChallengeDao.DuplicateException ex) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, ERROR_DUPLICATE_CHALLENGE);
        }

        return SUCCESS_CREATE_CHALLENGE;
    }
}
