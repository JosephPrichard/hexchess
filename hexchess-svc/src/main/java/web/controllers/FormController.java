package web.controllers;

import models.views.ServiceView;
import models.views.SessionView;
import services.broadcast.GroupBroadcaster;
import services.daos.ChallengeDao;
import services.daos.UserDao;
import models.common.ColorSelect;
import models.common.TimeControl;
import models.entities.ChallengeEntity;
import models.state.Player;
import models.entities.UserEntity;
import services.game.GameService;
import services.daos.DictionaryDao;
import io.jooby.*;
import io.jooby.exception.StatusCodeException;
import org.jsoup.Jsoup;
import web.reusable.AuthService;
import web.State;

import java.util.concurrent.CompletableFuture;

import static utils.Globals.*;
import static web.WebConstants.*;
import static services.daos.UserDao.*;

public class FormController extends Jooby {

    private final UserDao userDao;
    private final ChallengeDao challengeDao;
    private final DictionaryDao dictionaryDao;
    private final GameService gameService;
    private final AuthService authService;
    private final GroupBroadcaster userBroadcaster;

    public FormController(State state) {
        userDao = state.getUserDao();
        challengeDao = state.getChallengeDao();
        dictionaryDao = state.getDictionaryDao();
        gameService = state.getGameService();
        authService = state.getAuthService();
        userBroadcaster = state.getUserBroadcaster();

        setWorker(EXECUTOR);

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

    public record RegisterBody(String username, String password, String confirmPassword) {}

    public SessionView register(Context ctx) {
        RegisterBody body = ctx.body(RegisterBody.class);
        String username = body.username();
        String password = body.password();
        String confirmPassword = body.confirmPassword();

        if (username.length() < 5 || username.length() > 20) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, ERROR_INVALID_USERNAME);
        }
        if (!Jsoup.isValid(username, HTML_SAFELIST)) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, ERROR_UNSAFE_USERNAME);
        }
        validatePassword(password, confirmPassword);

        try {
            UserEntity user = userDao.insert(username, password);
            dictionaryDao.incrLeaderboardUser(user.getId(), user.getElo());

            String sessionId = authService.createSessionId();
            Cookie cookie = authService.createSessionCookie(sessionId);
            ctx.setResponseCookie(cookie);

            Player player = new Player(user.getId(), user.getUsername(), user.getCountry(), user.getElo());
            dictionaryDao.setSession(sessionId, player, cookie.getMaxAge());

            LOG.info("Registered a new selfPlayer={}", player);

            VerifiedUser verifiedUser = new VerifiedUser(player.getId(), player.getName(), player.getCountry(), player.getElo());

            return SessionView.fromUser(verifiedUser, cookie.getMaxAge());
        } catch (UserDao.TakenUsernameException ex) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, ERROR_DUPLICATE_USERNAME);
        }
    }

    public record LoginBody(String username, String password) {}

    public SessionView login(Context ctx) {
        LoginBody body = ctx.body(LoginBody.class);
        String username = body.username();
        String password = body.password();

        VerifiedUser verifiedUser = userDao.verify(username, password);
        if (verifiedUser == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, ERROR_INVALID_LOGIN);
        }

        String sessionId = authService.createSessionId();
        Cookie cookie = authService.createSessionCookie(sessionId);
        ctx.setResponseCookie(cookie);
        dictionaryDao.setSession(
            sessionId,
            new Player(verifiedUser.getId(), verifiedUser.getUsername(), verifiedUser.getCountry(), verifiedUser.getElo()),
            cookie.getMaxAge());

        LOG.info("Player has logged in {}", verifiedUser);

        return SessionView.fromUser(verifiedUser, cookie.getMaxAge());
    }

    public record UpdatePasswordBody(String password, String newPassword, String confirmNewPassword) {}

    public ServiceView updatePassword(Context ctx) {
        UpdatePasswordBody body = ctx.body(UpdatePasswordBody.class);
        String password = body.password();
        String newPassword = body.newPassword();
        String confirmNewPassword = body.confirmNewPassword();

        validatePassword(newPassword, confirmNewPassword);

        Player player = authService.getSessionPlayer(ctx);

        VerifiedUser verifiedUser = userDao.verify(player.getName(), password);
        if (verifiedUser == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, ERROR_INVALID_LOGIN);
        }

        userDao.updatePassword(verifiedUser.getId(), newPassword);

        return ServiceView.SUCCESS;
    }

    public record UpdateUserBody(String newUsername, String newCountry, String newBio) {}

    public SessionView updateUser(Context ctx) {
        UpdateUserBody body = ctx.body(UpdateUserBody.class);
        String newUsername = body.newUsername();
        String newCountry = body.newCountry();
        String newBio = body.newBio();

        Player player = authService.getSessionPlayer(ctx);

        VerifiedUser verifiedUser = userDao.updateUser(player.getId(), newUsername, newCountry, newBio);

        return SessionView.fromUser(verifiedUser, null);
    }

    public record TempSessionBody(String sessionId) {}

    public TempSessionBody createTempSession(Context ctx) {
        Player player = authService.getSessionPlayer(ctx);
        String tempSessionId = authService.createSessionId();

        dictionaryDao.setSession(tempSessionId, player, DictionaryDao.TEMP_SESSION_EXPIRE.toSeconds());

        return new TempSessionBody(tempSessionId);
    }

    public record RefreshResp(SessionView session) {
        public static final RefreshResp EMPTY = new RefreshResp(null);
    }

    public RefreshResp refreshSession(Context ctx) {
        String sessionId = AuthService.parseSession(ctx);
        if (sessionId == null) {
            return RefreshResp.EMPTY;
        }
        Player player = dictionaryDao.getSession(sessionId);
        if (player == null) {
            return RefreshResp.EMPTY;
        }

        Cookie cookie = authService.createSessionCookie(sessionId);
        ctx.setResponseCookie(cookie);
        dictionaryDao.updateSessionEx(sessionId, cookie.getMaxAge());

        LOG.info("Refreshed session for player={}", player.getId());

        return new RefreshResp(SessionView.fromPlayer(player, cookie.getMaxAge()));
    }

    public ServiceView logout(Context ctx) {
        String sessionId = AuthService.parseSession(ctx);

        dictionaryDao.deleteSession(sessionId);
        ctx.setResponseCookie(authService.createEmptyCookie());

        LOG.info("Logged out sessionId={}", sessionId);

        return ServiceView.SUCCESS;
    }

    public record CreateGameBody(ColorSelect firstColor, TimeControl timeControl) {}

    public record CreateGameResp(String gameId) {}

    public CreateGameResp createGame(Context ctx) {
        CreateGameBody body = ctx.body(CreateGameBody.class);

        ColorSelect firstColor = body.firstColor() != null ? body.firstColor() : ColorSelect.RANDOM;
        TimeControl timeControl = body.timeControl() != null ? body.timeControl() : TimeControl.UNLIMITED;

        String gameId = gameService.create(firstColor, timeControl);
        return new CreateGameResp(gameId);
    }

    public enum UpdtAction {
        ACCEPT,
        REJECT,
        DELETE
    }

    public record UpdateChallengeBody(long challengeeId, long challengerId, UpdtAction action) {}

    public record UpdateChallengeResp(String gameId) {}

    public UpdateChallengeResp updateChallenge(Context ctx) {
        UpdateChallengeBody body = ctx.body(UpdateChallengeBody.class);
        long challengeeId = body.challengeeId();
        long challengerId = body.challengerId();
        UpdtAction action = body.action();

        Player player = authService.getSessionPlayer(ctx);

        long callerId = player.getId();

        long targetId = switch (action) {
            case UpdtAction.ACCEPT, UpdtAction.REJECT -> challengeeId;
            case UpdtAction.DELETE -> challengerId;
        };

        String gameId = null;

        if (callerId == targetId) {
            ChallengeDao.DeleteResult result = challengeDao.delete(challengerId, challengeeId);
            if (result == null) {
                throw new StatusCodeException(StatusCode.NOT_FOUND, ERROR_NOT_FOUND_CHALLENGE);
            }
            if (action.equals(UpdtAction.ACCEPT)) {
                gameId = gameService.create(
                    ColorSelect.fromString(result.getStartColor()),
                    TimeControl.fromString(result.getTimeControl()));
            }
        } else {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, ERROR_UPDATE_CHALLENGE);
        }

        ctx.setResponseCode(StatusCode.OK);
        return new UpdateChallengeResp(gameId);
    }

    public void broadcastChallenge(ChallengeEntity entity) {
        try {
            String groupId = Long.toString(entity.getChallengeeId());
            LOG.info("Broadcasting challenge={} with groupId={} to user broadcaster", entity, groupId);

            byte[] output = JSON.writeValueAsBytes(entity);
            userBroadcaster.broadcast(groupId, output);
        } catch (Exception ex) {
            LOG.error("Error occurred while broadcasting challenge to user", ex);
        }
    }

    public void dispatchBroadcastChallenge(ChallengeEntity entity) {
        CompletableFuture.runAsync(() -> broadcastChallenge(entity), EXECUTOR);
    }

    public void dispatchDeleteExpired(long challengeeId) {
        CompletableFuture.runAsync(() -> challengeDao.deleteExpired(challengeeId), EXECUTOR);
    }

    public record CreateChallengeBody(long challengeeId, TimeControl timeControl, ColorSelect startColor) {}

    private void validateCreateChallenge(CreateChallengeBody body) {
        if (body.timeControl() == null) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, ERROR_INVALID_REQUEST);
        }
        if (body.startColor() == null) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, ERROR_INVALID_REQUEST);
        }
    }

    public ServiceView createChallenge(Context ctx) {
        CreateChallengeBody body = ctx.body(CreateChallengeBody.class);
        validateCreateChallenge(body);

        long challengeeId = body.challengeeId();
        ColorSelect startColor = body.startColor();
        TimeControl timeControl = body.timeControl();

        Player player = authService.getSessionPlayer(ctx);

        try {
            ChallengeEntity entity = challengeDao.insert(player.getId(), challengeeId, timeControl.toString(), startColor.toString());
            dispatchBroadcastChallenge(entity);
            dispatchDeleteExpired(player.getId());

            return ServiceView.SUCCESS;
        } catch (ChallengeDao.ParticipantException ex) {
            throw new StatusCodeException(StatusCode.NOT_FOUND, ERROR_NOT_FOUND_USER);
        } catch (ChallengeDao.SelfException ex) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, ERROR_SELF_CHALLENGE);
        } catch (ChallengeDao.DuplicateException ex) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, ERROR_DUPLICATE_CHALLENGE);
        }
    }
}