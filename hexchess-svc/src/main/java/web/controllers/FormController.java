package web.controllers;

import lombok.*;
import models.views.SessionView;
import services.daos.ChallengeDao;
import services.daos.UserDao;
import models.common.ColorSelect;
import models.common.TimeControl;
import models.entities.ChallengeEntity;
import models.entities.PlayerEntity;
import models.entities.UserEntity;
import services.game.GameService;
import services.daos.DictionaryDao;
import io.jooby.*;
import io.jooby.exception.StatusCodeException;
import org.jsoup.Jsoup;
import services.producers.ChallengeProducer;
import models.message.ChallengeMsg;
import web.reusable.AuthService;
import web.State;

import static utils.Globals.*;
import static web.WebConstants.*;
import static services.daos.UserDao.*;

public class FormController extends Jooby {

    private final UserDao userDao;
    private final ChallengeDao challengeDao;
    private final DictionaryDao dictionaryDao;
    private final GameService gameService;
    private final AuthService authService;
    private final ChallengeProducer challengeProducer;

    public FormController(State state) {
        userDao = state.getUserDao();
        challengeDao = state.getChallengeDao();
        dictionaryDao = state.getDictionaryDao();
        gameService = state.getGameService();
        authService = state.getAuthService();
        challengeProducer = state.getChallengeProducer();

        setWorker(EXECUTOR);

        error((ctx, cause, statusCode) -> {
            ctx.setResponseCode(statusCode);
            if (statusCode.value() == 500) {
                LOGGER.error("Error: {} ", statusCode, cause);
                ctx.send(ERROR_UNKNOWN);
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
        post("/forms/session/refresh", this::refreshSession);
        post("/forms/session/temp", this::createTempSession);
        post("/forms/logout", this::logout);
        post("/forms/games/create", this::createGame);
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

    @ToString
    @NoArgsConstructor
    @AllArgsConstructor
    public static class RegisterBody {
        public String username;
        public String password;
        public String confirmPassword;
    }

    public SessionView register(Context ctx) {
        RegisterBody body = ctx.body(RegisterBody.class);
        String username = body.username;
        String password = body.password;
        String confirmPassword = body.confirmPassword;

        if (username.length() < 5 || username.length() > 20) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, ERROR_INVALID_USERNAME);
        }
        if (!Jsoup.isValid(username, HTML_SAFELIST)) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, ERROR_UNSAFE_USERNAME);
        }
        validatePassword(password, confirmPassword);

        try {
            UserEntity user = userDao.insert(username, password);
            dictionaryDao.incrLeaderboardUser(user.id, user.elo);

            String sessionId = authService.createSessionId();
            Cookie cookie = authService.createSessionCookie(sessionId);
            ctx.setResponseCookie(cookie);

            PlayerEntity player = new PlayerEntity(user.id, user.username, user.country, user.elo);
            dictionaryDao.setSession(sessionId, player, cookie.getMaxAge());

            LOGGER.info("Registered a new player={}", player);

            VerifiedUser verifiedUser = new VerifiedUser(player.id, player.name, player.country, player.elo);

            return SessionView.fromUser(verifiedUser, cookie.getMaxAge());
        } catch (UserDao.TakenUsernameException ex) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, ERROR_DUPLICATE_USERNAME);
        }
    }

    @ToString
    @NoArgsConstructor
    @AllArgsConstructor
    public static class LoginBody {
        public String username;
        public String password;
    }

    public SessionView login(Context ctx) {
        LoginBody body = ctx.body(LoginBody.class);
        String username = body.username;
        String password = body.password;

        VerifiedUser verifiedUser = userDao.verify(username, password);
        if (verifiedUser == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, ERROR_INVALID_LOGIN);
        }

        String sessionId = authService.createSessionId();
        Cookie cookie = authService.createSessionCookie(sessionId);
        ctx.setResponseCookie(cookie);
        dictionaryDao.setSession(
                sessionId,
                new PlayerEntity(verifiedUser.id, verifiedUser.username, verifiedUser.country, verifiedUser.elo),
                cookie.getMaxAge());

        LOGGER.info("Player has logged in {}", verifiedUser);

        return SessionView.fromUser(verifiedUser, cookie.getMaxAge());
    }

    @ToString
    @NoArgsConstructor
    @AllArgsConstructor
    public static class UpdatePasswordBody {
        public String password;
        public String newPassword;
        public String confirmNewPassword;
    }

    public String updatePassword(Context ctx) {
        UpdatePasswordBody body = ctx.body(UpdatePasswordBody.class);
        String password = body.password;
        String newPassword = body.newPassword;
        String confirmNewPassword = body.confirmNewPassword;

        validatePassword(newPassword, confirmNewPassword);

        PlayerEntity player = authService.getSessionPlayer(ctx);

        VerifiedUser verifiedUser = userDao.verify(player.name, password);
        if (verifiedUser == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, ERROR_INVALID_LOGIN);
        }

        userDao.updatePassword(verifiedUser.id, newPassword);

        return "SUCCESS";
    }

    @ToString
    @NoArgsConstructor
    @AllArgsConstructor
    public static class UpdateUserBody {
        public String newUsername;
        public String newCountry;
        public String newBio;
    }

    public SessionView updateUser(Context ctx) {
        UpdateUserBody body = ctx.body(UpdateUserBody.class);
        String newUsername = body.newUsername;
        String newCountry = body.newCountry;
        String newBio = body.newBio;

        PlayerEntity player = authService.getSessionPlayer(ctx);

        VerifiedUser verifiedUser = userDao.updateUser(player.id, newUsername, newCountry, newBio);

        return SessionView.fromUser(verifiedUser, null);
    }

    public String createTempSession(Context ctx) {
        PlayerEntity player = authService.getSessionPlayer(ctx);
        String tempSessionId = authService.createSessionId();
        dictionaryDao.setSession(tempSessionId, player, DictionaryDao.TEMP_SESSION_EXPIRE.toSeconds());
        return tempSessionId;
    }

    @ToString
    @NoArgsConstructor
    @AllArgsConstructor
    public static class RefreshResp {
        public static final RefreshResp EMPTY = new RefreshResp();

        public SessionView session;
    }

    public RefreshResp refreshSession(Context ctx) {
        String sessionId = authService.parseSession(ctx);
        if (sessionId == null) {
            return RefreshResp.EMPTY;
        }
        PlayerEntity player = authService.getSessionPlayer(ctx);
        if (player == null) {
            return RefreshResp.EMPTY;
        }

        Cookie cookie = authService.createSessionCookie(sessionId);
        ctx.setResponseCookie(cookie);
        dictionaryDao.updateSessionEx(sessionId, cookie.getMaxAge());

        LOGGER.info("Refreshed session for player={}", player.id);

        return new RefreshResp(SessionView.fromPlayer(player, cookie.getMaxAge()));
    }

    public String logout(Context ctx) {
        String sessionId = authService.parseSession(ctx);

        dictionaryDao.deleteSession(sessionId);
        ctx.setResponseCookie(authService.createEmptyCookie());

        LOGGER.info("Logged out sessionId={}", sessionId);

        return "SUCCESS";
    }

    @ToString
    @NoArgsConstructor
    @AllArgsConstructor
    public static class CreateGameBody {
        public ColorSelect firstColor;
        public TimeControl timeControl;
    }

    public String createGame(Context ctx) {
        CreateGameBody body = ctx.body(CreateGameBody.class);

        if (body.firstColor == null) {
            body.firstColor = ColorSelect.RANDOM;
        }
        if (body.timeControl == null) {
            body.timeControl = TimeControl.UNLIMITED;
        }

        String gameId = gameService.create(body.firstColor, body.timeControl);

        ctx.setResponseType(MediaType.TEXT);
        return gameId;
    }

    @ToString
    @NoArgsConstructor
    @AllArgsConstructor
    public static class UpdateChallengeBody {
        public long challengeeId;
        public long challengerId;
        public String action;
    }

    @ToString
    @EqualsAndHashCode
    @AllArgsConstructor
    public static class UpdateChallengeResp {
        public String gameId;
    }

    public UpdateChallengeResp updateChallenge(Context ctx) {
        UpdateChallengeBody body = ctx.body(UpdateChallengeBody.class);
        long challengeeId = body.challengeeId;
        long challengerId = body.challengerId;
        String action = body.action.toUpperCase();

        PlayerEntity player = authService.getSessionPlayer(ctx);

        long callerId = player.id;

        long targetId = switch (action) {
            case "ACCEPT", "REJECT" -> challengeeId;
            case "DELETE" -> challengerId;
            default -> throw new StatusCodeException(StatusCode.BAD_REQUEST, ERROR_INVALID_CHALLENGE_ACTION);
        };

        String gameId = null;

        if (callerId == targetId) {
            ChallengeDao.DeleteResult result = challengeDao.delete(challengerId, challengeeId);
            if (result == null) {
                throw new StatusCodeException(StatusCode.NOT_FOUND, ERROR_NOT_FOUND_CHALLENGE);
            }
            if (action.equals("ACCEPT")) {
                gameId = gameService.create(
                    ColorSelect.fromString(result.startColor),
                    TimeControl.fromString(result.timeControl));
            }
        } else {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, ERROR_UPDATE_CHALLENGE);
        }

        ctx.setResponseCode(StatusCode.OK);
        return new UpdateChallengeResp(gameId);
    }

    @ToString
    @NoArgsConstructor
    @AllArgsConstructor
    public static class CreateChallengeBody {
        public long challengeeId;
        public TimeControl timeControl;
        public ColorSelect startColor;
    }

    private void validateCreateChallenge(CreateChallengeBody body) {
        if (body.timeControl == null) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, ERROR_INVALID_REQUEST);
        }
        if (body.startColor == null) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, ERROR_INVALID_REQUEST);
        }
    }

    public String createChallenge(Context ctx) {
        CreateChallengeBody body = ctx.body(CreateChallengeBody.class);
        validateCreateChallenge(body);

        long challengeeId = body.challengeeId;
        ColorSelect startColor = body.startColor;
        TimeControl timeControl = body.timeControl;

        String sessionId = authService.parseSession(ctx);
        if (sessionId == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, ERROR_REQUIRED_LOGIN);
        }
        PlayerEntity player = dictionaryDao.getSession(sessionId);
        if (player == null) {
            ctx.setResponseCookie(authService.createEmptyCookie());
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, ERROR_SESSION_EXPIRED);
        }

        try {
            ChallengeEntity entity = challengeDao.insert(player.id, challengeeId, timeControl.toString(), startColor.toString());
            EXECUTOR.execute(() -> challengeProducer.broadcastChallenge(ChallengeMsg.fromEntity(entity)));
            EXECUTOR.execute(() -> challengeDao.deleteExpired(player.id));

            return "SUCCESS";
        } catch (ChallengeDao.ParticipantException ex) {
            throw new StatusCodeException(StatusCode.NOT_FOUND, ERROR_NOT_FOUND_USER);
        } catch (ChallengeDao.SelfException ex) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, ERROR_SELF_CHALLENGE);
        } catch (ChallengeDao.DuplicateException ex) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, ERROR_DUPLICATE_CHALLENGE);
        }
    }
}
