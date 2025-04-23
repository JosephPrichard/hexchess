package web.controllers;

import com.fasterxml.jackson.core.JsonProcessingException;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;
import daos.ChallengeDao;
import daos.UserDao;
import models.ChallengeEntity;
import models.UserEntity;
import services.Broadcaster;
import services.GameService;
import services.RemoteDict;
import io.jooby.*;
import io.jooby.exception.StatusCodeException;
import models.PlayerEntity;
import org.jsoup.Jsoup;
import web.reusable.SessionService;
import web.State;

import java.io.IOException;

import static utils.Globals.*;
import static daos.UserDao.*;
import static web.reusable.SessionService.*;
import static web.WebConstants.*;

public class FormController extends Jooby {

    private final UserDao userDao;
    private final ChallengeDao challengeDao;
    private final RemoteDict remoteDict;
    private final GameService gameService;
    private final SessionService sessionService;
    private final Broadcaster userBroadcaster;

    public FormController(State state) {
        userDao = state.getUserDao();
        challengeDao = state.getChallengeDao();
        remoteDict = state.getRemoteDict();
        gameService = state.getGameService();
        sessionService = state.getSessionService();
        userBroadcaster = state.getUserBroadcaster();

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
    @AllArgsConstructor
    public static class RegisterBody {
        String username;
        String password;
        String confirmPassword;
    }

    public String register(Context ctx) {
        RegisterBody body = ctx.body(RegisterBody.class);
        String username = body.username;
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
            remoteDict.incrLeaderboardUser(user.id, user.elo);

            String sessionId = sessionService.createId();
            Cookie cookie = sessionService.createCookie(sessionId, user.id, user.username, user.country);
            ctx.setResponseCookie(cookie);

            PlayerEntity player = new PlayerEntity(user.id, user.username, user.country, user.elo);
            remoteDict.setSession(sessionId, player, cookie.getMaxAge());

            LOGGER.info("Registered a new player={}", player);

            return SUCCESS_REGISTER;
        } catch (UserDao.TakenUsernameException ex) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, ERROR_DUPLICATE_USERNAME);
        }
    }

    @Data
    @NoArgsConstructor
    @AllArgsConstructor
    public static class LoginBody {
        String username;
        String password;
    }

    public String login(Context ctx) {
        LoginBody body = ctx.body(LoginBody.class);
        String username = body.username;
        String password = body.getPassword();

        VerifiedUser verifiedUser = userDao.verify(username, password);
        if (verifiedUser == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, ERROR_INVALID_LOGIN);
        }

        String sessionId = sessionService.createId();
        Cookie cookie = sessionService.createCookie(sessionId, verifiedUser.id, verifiedUser.username, verifiedUser.country);
        ctx.setResponseCookie(cookie);
        remoteDict.setSession(
            sessionId,
            new PlayerEntity(verifiedUser.id, verifiedUser.username, verifiedUser.country, verifiedUser.elo),
            cookie.getMaxAge());

        LOGGER.info("Player has logged in {}", verifiedUser);

        return SUCCESS_LOGIN;
    }

    @Data
    @NoArgsConstructor
    @AllArgsConstructor
    public static class UpdatePasswordBody {
        String password;
        String newPassword;
        String confirmNewPassword;
    }

    public String updatePassword(Context ctx) {
        UpdatePasswordBody body = ctx.body(UpdatePasswordBody.class);
        String password = body.getPassword();
        String newPassword = body.getNewPassword();
        String confirmNewPassword = body.getConfirmNewPassword();

        validatePassword(newPassword, confirmNewPassword);

        SessionValue session = sessionService.parseSession(ctx);
        if (session == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, ERROR_REQUIRED_LOGIN);
        }
        VerifiedUser player = userDao.verify(session.username, password);
        if (player == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, ERROR_INVALID_LOGIN);
        }

        userDao.updatePassword(player.id, newPassword);

        return SUCCESS_UPDATE_PASSWORD;
    }

    @Data
    @NoArgsConstructor
    @AllArgsConstructor
    public static class UpdateUserBody {
        String newUsername;
        String newCountry;
        String newBio;
    }

    public String updateUser(Context ctx) {
        UpdateUserBody body = ctx.body(UpdateUserBody.class);
        String newUsername = body.getNewUsername();
        String newCountry = body.getNewCountry();
        String newBio = body.getNewBio();

        SessionValue session = sessionService.parseSession(ctx);
        if (session == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, ERROR_REQUIRED_LOGIN);
        }
        PlayerEntity player = remoteDict.getSession(session.sessionId);
        if (player == null) {
            ctx.setResponseCookie(sessionService.createEmptyCookie());
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, ERROR_SESSION_EXPIRED);
        }

        VerifiedUser verifiedUser = userDao.updateUser(player.id, newUsername, newCountry, newBio);
        if (verifiedUser != null) {
            Cookie cookie = sessionService.createCookie(session.sessionId, verifiedUser.id, verifiedUser.username, verifiedUser.country);
            ctx.setResponseCookie(cookie);
        }

        return SUCCESS_UPDATE_USER;
    }

    public String updateSession(Context ctx) {
        SessionValue session = sessionService.parseSession(ctx);
        if (session != null) {
            Cookie cookie = sessionService.createCookie(session.sessionId, session.userId, session.username, session.country);
            ctx.setResponseCookie(cookie);
            remoteDict.updateSessionEx(session.sessionId, cookie.getMaxAge());

            LOGGER.info("Refreshed session for user={}", session.userId);
        }

        return SUCCESS_GENERIC;
    }

    public String logout(Context ctx) {
        SessionValue session = sessionService.parseSession(ctx);
        remoteDict.deleteSession(session.sessionId);
        ctx.setResponseCookie(sessionService.createEmptyCookie());

        LOGGER.info("Logged out user={}", session.userId);

        return SUCCESS_GENERIC;
    }

    @Data
    @NoArgsConstructor
    @AllArgsConstructor
    public static class CreateGameBody {
        String color;
    }

    public String createGame(Context ctx) {
        CreateGameBody body = ctx.body(CreateGameBody.class);
        String color = body.getColor();

        Boolean isFirstWhite = null;
        if (color != null && color.equals("white")) {
            isFirstWhite = true;
        } else if (color != null && color.equals("black")) {
            isFirstWhite = false;
        }

        ctx.setResponseType(MediaType.TEXT);
        return gameService.create(isFirstWhite);
    }

    @Data
    @NoArgsConstructor
    @AllArgsConstructor
    public static class UpdateChallengeBody {
        long challengeeId;
        long challengerId;
        String action;
    }

    @Data
    @AllArgsConstructor
    public static class UpdateChallengeResp {
        String code;
        String gameId;

        static String json(String code) throws JsonProcessingException {
            return JSON_MAPPER.writeValueAsString(new UpdateChallengeResp(code, null));
        }
    }

    public UpdateChallengeResp updateChallenge(Context ctx) throws IOException {
        UpdateChallengeBody body = ctx.body(UpdateChallengeBody.class);
        long challengeeId = body.getChallengeeId();
        long challengerId = body.getChallengerId();
        String action = body.getAction().toUpperCase();

        SessionService.SessionValue session = sessionService.parseSession(ctx);
        if (session == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, UpdateChallengeResp.json(ERROR_REQUIRED_LOGIN));
        }
        PlayerEntity player = remoteDict.getSession(session.sessionId);
        if (player == null) {
            ctx.setResponseCookie(sessionService.createEmptyCookie());
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, UpdateChallengeResp.json(ERROR_SESSION_EXPIRED));
        }

        long callerId = player.id;

        long targetId = switch (action) {
            case "ACCEPT", "REJECT" -> challengeeId;
            case "DELETE" -> challengerId;
            default ->
                throw new StatusCodeException(StatusCode.BAD_REQUEST, UpdateChallengeResp.json(ERROR_INVALID_CHALLENGE_ACTION));
        };

        String gameId = null;

        if (callerId == targetId) {
            int count = challengeDao.delete(challengerId, challengeeId);
            if (count == 0) {
                throw new StatusCodeException(StatusCode.NOT_FOUND, UpdateChallengeResp.json(ERROR_NOT_FOUND_CHALLENGE));
            }
            if (action.equals("ACCEPT")) {
                gameId = gameService.create(null);
            }
        } else {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, UpdateChallengeResp.json(ERROR_UPDATE_CHALLENGE));
        }

        ctx.setResponseCode(StatusCode.OK);
        return new UpdateChallengeResp(SUCCESS_UPDATE_CHALLENGE, gameId);
    }

    @Data
    @NoArgsConstructor
    @AllArgsConstructor
    public static class CreateChallengeBody {
        long challengeeId;
    }

    public String createChallenge(Context ctx) {
        CreateChallengeBody body = ctx.body(CreateChallengeBody.class);
        long challengeeId = body.getChallengeeId();

        SessionService.SessionValue session = sessionService.parseSession(ctx);
        if (session == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, ERROR_REQUIRED_LOGIN);
        }
        PlayerEntity player = remoteDict.getSession(session.sessionId);
        if (player == null) {
            ctx.setResponseCookie(sessionService.createEmptyCookie());
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, ERROR_SESSION_EXPIRED);
        }

        try {
            ChallengeEntity entity = challengeDao.insert(player.id, challengeeId);

            EXECUTOR.execute(() -> {
                try {
                    String jsonOutput = JSON_MAPPER.writeValueAsString(entity);
                    userBroadcaster.broadcast(Long.toString(challengeeId), jsonOutput);
                } catch (JsonProcessingException e) {
                    LOGGER.error("Error occurred while broadcasting challenge to user", e);
                }
            });
            EXECUTOR.execute(() -> challengeDao.deleteExpired(session.userId));
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
