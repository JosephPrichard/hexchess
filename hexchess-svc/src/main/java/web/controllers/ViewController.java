package web.controllers;

import chess.ChessBoard;
import chess.PieceMove;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.core.type.TypeReference;
import models.entities.RankedEntity;
import models.state.Player;
import services.daos.ChallengeDao;
import services.daos.ReplayDao;
import services.daos.UserDao;
import io.jooby.exception.BadRequestException;
import models.entities.*;
import models.views.*;
import services.game.GameService;
import services.daos.DictionaryDao;
import io.jooby.*;
import web.reusable.AuthService;
import web.State;

import java.time.Duration;
import java.util.List;
import java.util.concurrent.CompletableFuture;
import java.util.stream.Collectors;

import static utils.Globals.*;
import static web.WebConstants.*;
import static services.daos.DictionaryDao.*;

public class ViewController extends Jooby {

    private static final TypeReference<List<PieceMove>> MOVE_LIST_TYPE = new TypeReference<>() {};
    public static final int PER_PAGE = 25;
    public static final String LONG_CACHE_CONTROL = String.format("public, max-age=%s, immutable", Duration.ofDays(1).toSeconds());

    private final UserDao userDao;
    private final ReplayDao replayDao;
    private final ChallengeDao challengeDao;
    private final DictionaryDao dictionaryDao;
    private final GameService gameService;
    private final AuthService authService;
    private final List<String> countryList;
    private final ChessBoard initialBoard;

    public ViewController(State state) {
        userDao = state.getUserDao();
        replayDao = state.getReplayDao();
        challengeDao = state.getChallengeDao();
        dictionaryDao = state.getDictionaryDao();
        gameService = state.getGameService();
        authService = state.getAuthService();
        countryList = state.getCountryList();
        initialBoard = state.getInitialBoard();

        setWorker(EXECUTOR);

        error(this::handleError);

//        use(next -> ctx -> {
//            ctx.setResponseType(MediaType.JSON);
//            return next.apply(ctx);
//        });

        get("/views/players/self", this::getSelf);
        get("/views/players/{id}", this::getPlayer);
        get("/views/players/search", this::searchPlayers);
        get("/views/leaderboard", this::getLeaderboard);
        get("/views/replay/{id}", this::getReplay);
        get("/views/replay/{id}/move-list", this::getReplayMoveList);
        get("/views/replays", this::getReplayList);
        get("/views/challenges", this::getChallengeList);
        get("/views/countries", this::getCountryList);
        get("/views/initial-board", this::getInitialBoard);
    }

    public Player authenticate(Context ctx) {
        String sessionId = authService.parseSession(ctx);
        if (sessionId == null) {
            throw new BadRequestException(ERROR_REQUIRED_LOGIN);
        }
        Player player = dictionaryDao.getSession(sessionId);
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
            LOG.error("Error: {}", statusCode, cause);
            errorMessage = ERROR_UNKNOWN;
        } else {
            String message = "Error: " + statusCode;
            LOG.error(message);
            errorMessage = message;
        }
        ctx.send(errorMessage);
    }

    public ChessBoard getInitialBoard(Context ctx) {
        ctx.setResponseHeader("Cache-Control", LONG_CACHE_CONTROL);
        return initialBoard;
    }

    public List<String> getCountryList(Context ctx) {
        ctx.setResponseHeader("Cache-Control", LONG_CACHE_CONTROL);
        return countryList;
    }

    public UserView getSelf(Context ctx) {
        Player player = authenticate(ctx);

        UserEntity entity = userDao.getById(player.getId());

        LOG.info("Retrieved self user={}", entity);

        return UserView.create(entity);
    }

    public record LeaderboardResp(int totalPages, List<UserView> userList) {}

    public LeaderboardResp getLeaderboard(Context ctx) {
        int page;
        try {
            page = ctx.query("page").toOptional().map(Integer::parseUnsignedInt).orElse(1);
        } catch (NumberFormatException ex) {
            LOG.warn("Page value is not valid integer");
            throw new BadRequestException(ERROR_INVALID_REQUEST);
        }

        Leaderboard leaderboard = dictionaryDao.getLeaderboardPage(page, PER_PAGE);
        List<UserEntity> entityList = userDao.getByRankedUsers(leaderboard.users());

        RankedEntity.joinRanks(leaderboard.users(), entityList);

        List<UserView> viewList = entityList.stream().map(UserView::create).toList();
        return new LeaderboardResp(leaderboard.pageCount(), viewList);
    }

    public record UserWithReplaysResp(UserView user, List<ReplayView> replayList) {}

    public UserWithReplaysResp getPlayer(Context ctx) throws Exception {
        String id = ctx.path("id").value();
        long userId;
        try {
            userId = Long.parseUnsignedLong(id);
        } catch (NumberFormatException ex) {
            LOG.warn("Id={} is not a valid long", id);
            throw new BadRequestException(ERROR_INVALID_REQUEST);
        }

        CompletableFuture<UserEntity> userFut = CompletableFuture.supplyAsync(() -> userDao.getById(userId), EXECUTOR);
        CompletableFuture<List<ReplayEntity>> replayListFut =
                CompletableFuture.supplyAsync(() -> replayDao.getUserReplays(userId, null, PER_PAGE), EXECUTOR);

        UserEntity userEntity = userFut.get();
        if (userEntity == null) {
            LOG.warn("User not found for id={}", userId);
            throw new BadRequestException(ERROR_NOT_FOUND_USER);
        }

        userEntity.setRank(dictionaryDao.getLeaderboardRank(userEntity.getId()));
        List<ReplayEntity> replayEntityList = replayListFut.get();

        UserView userView = UserView.create(userEntity);
        List<ReplayView> replayViewList = replayEntityList.stream().map(ReplayView::createRow).collect(Collectors.toList());

        return new UserWithReplaysResp(userView, replayViewList);
    }

    public List<UserView> searchPlayers(Context ctx) {
        int page;
        try {
            page = ctx.query("page").toOptional().map(Integer::parseUnsignedInt).orElse(1);
        } catch (NumberFormatException ex) {
            LOG.warn("Page value is not valid integer");
            throw new BadRequestException(ERROR_INVALID_REQUEST);
        }

        String name = ctx.query("username").value("");

        if (name.isEmpty()) {
            return List.of();
        }

        List<UserEntity> entityList = userDao.searchByName(name, page, PER_PAGE);
        return entityList.stream().map(UserView::create).toList();
    }

    public ReplayView getReplay(Context ctx) {
        String id = ctx.path("id").value();
        long replayId;
        try {
            replayId = Long.parseUnsignedLong(id);
        } catch (NumberFormatException ex) {
            LOG.warn("Id={} is not a valid long", id);
            throw new BadRequestException(ERROR_INVALID_REQUEST);
        }

        ReplayEntity entity = replayDao.getReplay(replayId);

        return ReplayView.createHeader(entity);
    }

    public List<PieceMove> getReplayMoveList(Context ctx) throws JsonProcessingException {
        String id = ctx.path("id").value();
        long replayId;
        try {
            replayId = Long.parseUnsignedLong(id);
        } catch (NumberFormatException ex) {
            LOG.warn("Id={} is not a valid long", id);
            throw new BadRequestException(ERROR_INVALID_REQUEST);
        }

        String moveListJson = replayDao.getReplayMoveList(replayId);

        ctx.setResponseHeader("Cache-Control", LONG_CACHE_CONTROL);
        return JSON.readValue(moveListJson, MOVE_LIST_TYPE);
    }

    public List<ChallengeView> getChallengeList(Context ctx) {
        String participants = ctx.query("participants").value("");

        Player player = authenticate(ctx);

        List<ChallengeEntity> entityList = switch (participants) {
            case "received" -> challengeDao.getByParticipant(null, player.getId());
            case "sent" -> challengeDao.getByParticipant(player.getId(), null);
            default -> {
                LOG.warn("Invalid participants value={} while getting challengeList", participants);
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
