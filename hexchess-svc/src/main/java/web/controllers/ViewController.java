package web.controllers;

import models.entities.RankedEntity;
import services.daos.ChallengeDao;
import services.daos.ReplayDao;
import services.daos.UserDao;
import io.jooby.exception.BadRequestException;
import models.entities.*;
import models.views.*;
import services.game.GameService;
import services.daos.DictionaryDao;
import io.jooby.*;
import lombok.AllArgsConstructor;
import lombok.Data;
import web.reusable.PathService;
import web.reusable.AuthService;
import web.State;

import java.io.IOException;
import java.time.Duration;
import java.util.List;
import java.util.concurrent.CompletableFuture;
import java.util.stream.Collectors;

import static utils.Globals.*;
import static web.WebConstants.*;
import static services.daos.DictionaryDao.*;

public class ViewController extends Jooby {

    public static final int PER_PAGE = 25;

    private final UserDao userDao;
    private final ReplayDao replayDao;
    private final ChallengeDao challengeDao;
    private final DictionaryDao dictionaryDao;
    private final GameService gameService;
    private final AuthService authService;
    private final PathService pathService;
    private final List<String> countryList;
    private final String initialBoardJson;

    public ViewController(State state) {
        userDao = state.getUserDao();
        replayDao = state.getReplayDao();
        challengeDao = state.getChallengeDao();
        dictionaryDao = state.getDictionaryDao();
        gameService = state.getGameService();
        authService = state.getAuthService();
        pathService = state.getPathService();
        countryList = state.getCountryList();
        initialBoardJson = state.getInitialBoardJson();

        setWorker(EXECUTOR);

        error(this::handleError);

        use(next -> ctx -> {
            ctx.setResponseType(MediaType.JSON);
            return next.apply(ctx);
        });

        get("/views/players/self", this::getSelf);
        get("/views/players/{id}", this::getPlayer);
        get("/views/players/search", this::searchPlayers);
        get("/views/leaderboard", this::getLeaderboard);
        get("/views/replay/{id}", this::getReplay);
        get("/views/replays", this::getReplayList);
        get("/views/challenges", this::getChallengeList);
        get("/views/countries", this::getCountryList);
        get("/views/initial-board", this::getInitialBoard);
    }

    public PlayerEntity authenticate(Context ctx) {
        String sessionId = authService.parseSession(ctx);
        if (sessionId == null) {
            throw new BadRequestException(ERROR_REQUIRED_LOGIN);
        }
        PlayerEntity player = dictionaryDao.getSession(sessionId);
        if (player == null) {
            ctx.setResponseCookie(authService.createEmptyCookie());
            throw new BadRequestException(ERROR_SESSION_EXPIRED);
        }
        return player;
    }

    public void handleError(Context ctx, Throwable cause, StatusCode statusCode) {
        String errorMessage;

        ctx.setResponseType(MediaType.TEXT);
        ctx.setResponseCode(statusCode);

        if (statusCode.value() == 500) {
            LOGGER.error("Error: {}", statusCode, cause);
            errorMessage = ERROR_UNKNOWN;
        } else {
            String message = "Error: " + statusCode;
            LOGGER.error(message);
            errorMessage = message;
        }
        ctx.send(errorMessage);
    }

    public String getInitialBoard(Context ctx) {
        ctx.setResponseHeader("Cache-Control", String.format("public, max-age=%s, immutable", Duration.ofDays(1).toSeconds()));
        return initialBoardJson;
    }

    public List<String> getCountryList(Context ctx) {
        ctx.setResponseHeader("Cache-Control", String.format("public, max-age=%s, immutable", Duration.ofDays(1).toSeconds()));
        return countryList;
    }

    public UserView getSelf(Context ctx) throws IOException {
        PlayerEntity player = authenticate(ctx);

        UserEntity entity = userDao.getById(player.id);

        LOGGER.info("Retrieved self user={}", entity);

        return UserView.create(entity);
    }

    @Data
    @AllArgsConstructor
    public static class LeaderboardResp {
        public int totalPages;
        public List<UserView> userList;
    }

    public LeaderboardResp getLeaderboard(Context ctx) {
        int page = pathService.getPageParam(ctx);

        Leaderboard leaderboard = dictionaryDao.getLeaderboardPage(page, PER_PAGE);
        List<UserEntity> entityList = userDao.getByRankedUsers(leaderboard.users);

        RankedEntity.joinRanks(leaderboard.users, entityList);

        List<UserView> viewList = entityList.stream().map(UserView::create).toList();
        return new LeaderboardResp(leaderboard.pageCount, viewList);
    }

    @Data
    @AllArgsConstructor
    public static class UserWithReplaysResp {
        public UserView user;
        public List<ReplayView> replayList;
    }

    public UserWithReplaysResp getPlayer(Context ctx) throws Exception {
        long userId = pathService.getPathAsLong(ctx, "id");

        CompletableFuture<UserEntity> userFut = CompletableFuture.supplyAsync(() -> userDao.getById(userId), EXECUTOR);
        CompletableFuture<List<ReplayEntity>> replayListFut =
                CompletableFuture.supplyAsync(() -> replayDao.getUserReplays(userId, null, PER_PAGE), EXECUTOR);

        UserEntity userEntity = userFut.get();
        if (userEntity == null) {
            LOGGER.warn("User not found for id={}", userId);
            throw new BadRequestException(ERROR_NOT_FOUND_CHALLENGE);
        }

        userEntity.rank = dictionaryDao.getLeaderboardRank(userEntity.id);
        List<ReplayEntity> replayEntityList = replayListFut.get();

        UserView userView = UserView.create(userEntity);
        List<ReplayView> replayViewList = replayEntityList.stream().map(ReplayView::createRow).collect(Collectors.toList());

        return new UserWithReplaysResp(userView, replayViewList);
    }

    public List<UserView> searchPlayers(Context ctx) {
       int page = pathService.getPageParam(ctx);

        String name = ctx.query("username").value("");

        if (name.isEmpty()) {
            return List.of();
        }

        List<UserEntity> entityList = userDao.searchByName(name, page, PER_PAGE);
        return entityList.stream().map(UserView::create).toList();
    }

    public ReplayView getReplay(Context ctx) {
        long replayId = pathService.getPathAsLong(ctx, "id");

        ReplayEntity entity = replayDao.getReplay(replayId);

        return ReplayView.createHeader(entity);
    }

    public List<ChallengeView> getChallengeList(Context ctx) {
        String participants = ctx.query("participants").value("");

        PlayerEntity player = authenticate(ctx);

        List<ChallengeEntity> entityList = switch (participants) {
            case "received" -> challengeDao.getByParticipant(null, player.id);
            case "sent" -> challengeDao.getByParticipant(player.id, null);
            default -> {
                LOGGER.warn("Invalid participants value={} while getting challengeList", participants);
                throw new BadRequestException(ERROR_INVALID_PARTICIPANTS);
            }
        };

        return entityList.stream().map(ChallengeView::create).toList();
    }

    public List<ReplayView> getReplayList(Context ctx) {
        long userId = ctx.query("userId").longValue();
        Long afterId = ctx.query("afterId").toOptional().map(Long::parseUnsignedLong).orElse(null);

        List<ReplayEntity> entityList = replayDao.getUserReplays(userId, afterId, 25);
        if (entityList.isEmpty()) {
            return List.of();
        }

        return entityList.stream().map(ReplayView::createRow).toList();
    }
}
