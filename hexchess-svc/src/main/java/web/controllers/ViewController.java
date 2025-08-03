package web.controllers;

import chess.ChessBoard;
import models.entities.UserRankEntity;
import models.state.PlayerState;
import services.daos.*;
import io.jooby.exception.BadRequestException;
import models.entities.*;
import models.views.*;
import io.jooby.*;
import web.reusable.AuthService;
import web.State;

import java.time.Duration;
import java.util.List;
import java.util.Optional;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.Future;
import java.util.concurrent.TimeUnit;
import java.util.stream.Collectors;

import static utils.Globals.*;
import static web.WebConstants.*;
import static services.daos.DictionaryDao.*;

public class ViewController extends Jooby {

    public static final int PER_PAGE = 25;
    public static final String LONG_CACHE_CONTROL = String.format("public, max-age=%s, immutable", Duration.ofDays(1).toSeconds());

    private final UserDao userDao;
    private final ReplayDao replayDao;
    private final ChallengeDao challengeDao;
    private final DictionaryDao dictionaryDao;
    private final AuthService authService;
    private final List<String> countryList;
    private final ChessBoard initialBoard;

    public ViewController(State state) {
        userDao = state.getUserDao();
        replayDao = state.getReplayDao();
        challengeDao = state.getChallengeDao();
        dictionaryDao = state.getDictionaryDao();
        authService = state.getAuthService();
        countryList = state.getCountryList();
        initialBoard = state.getInitialBoard();

        setWorker(EXECUTOR);

        get("/views/players/self", this::getSelf);
        get("/views/players/{id}", this::getPlayer);
        get("/views/players/search", this::searchPlayers);
        get("/views/leaderboard", this::getLeaderboard);
        get("/views/replay/{id}", this::getReplay);
        get("/views/replay/{id}/moves", this::getReplayMoveList);
        get("/views/replays", this::getReplayList);
        get("/views/challenges", this::getChallengeList);
        get("/views/countries", this::getCountryList);
        get("/views/initial-board", this::getInitialBoard);
        get("/views/chess/rooms", this::getRoomLists);
    }

    public ChessBoard getInitialBoard(Context ctx) {
//        ctx.setResponseHeader("Cache-Control", LONG_CACHE_CONTROL);
        return initialBoard;
    }

    public List<String> getCountryList(Context ctx) {
//        ctx.setResponseHeader("Cache-Control", LONG_CACHE_CONTROL);
        return countryList;
    }

    public UserView getSelf(Context ctx) {
        PlayerState player = authService.getSessionPlayer(ctx);

        UserEntity entity = userDao.getById(player.getId());
        LOG.info("Retrieved self user={}", entity);

        return UserView.create(entity);
    }

    public record LeaderboardResp(int totalPages, List<UserView> userList) {}

    public LeaderboardResp getLeaderboard(Context ctx) {
        Optional<String> pageQuery = ctx.query("page").toOptional();

        int page;
        try {
            page = pageQuery.map(Integer::parseUnsignedInt).orElse(1);
        } catch (NumberFormatException ex) {
            LOG.warn("Query param 'page' is not a valid long", ex);
            throw new BadRequestException(ERROR_INVALID_REQUEST);
        }

        Leaderboard leaderboard = dictionaryDao.getLeaderboardPage(page, PER_PAGE);
        List<UserEntity> entityList = userDao.getByRankedUsers(leaderboard.users());

        UserRankEntity.joinRanks(leaderboard.users(), entityList);

        List<UserView> viewList = entityList.stream().map(UserView::create).toList();
        return new LeaderboardResp(leaderboard.pageCount(), viewList);
    }

    public Future<UserEntity> dispatchGetById(long userId) {
        return CompletableFuture.supplyAsync(() -> userDao.getById(userId), EXECUTOR);
    }

    public Future<List<ReplayEntity>> dispatchUserReplays(long userId) {
        return CompletableFuture.supplyAsync(() -> replayDao.getUserReplays(userId, null, PER_PAGE), EXECUTOR);
    }

    public record UserWithReplaysResp(UserView user, List<ReplayView> replayList) {}

    public UserWithReplaysResp getPlayer(Context ctx) throws Exception {
        String id = ctx.path("id").value();

        long userId;
        try {
            userId = Long.parseUnsignedLong(id);
        } catch (NumberFormatException ex) {
            LOG.warn("Path param 'id' is not a valid long", ex);
            throw new BadRequestException(ERROR_INVALID_REQUEST);
        }

        Future<UserEntity> userFut = dispatchGetById(userId);
        Future<List<ReplayEntity>> replayListFut = dispatchUserReplays(userId);

        UserEntity userEntity = userFut.get(MAX_WAIT_MS, TimeUnit.MILLISECONDS);
        if (userEntity == null) {
            LOG.warn("User not found for id={}", userId);
            throw new BadRequestException(ERROR_NOT_FOUND_USER);
        }

        userEntity.setRank(dictionaryDao.getLeaderboardRank(userEntity.getId()));
        List<ReplayEntity> replayEntityList = replayListFut.get(MAX_WAIT_MS, TimeUnit.MILLISECONDS);

        UserView userView = UserView.create(userEntity);
        List<ReplayView> replayViewList = replayEntityList.stream().map(ReplayView::createRow).collect(Collectors.toList());

        return new UserWithReplaysResp(userView, replayViewList);
    }

    public List<UserView> searchPlayers(Context ctx) {
        Optional<String> pageQuery = ctx.query("page").toOptional();

        int page;
        try {
            page = pageQuery.map(Integer::parseUnsignedInt).orElse(1);
        } catch (NumberFormatException ex) {
            LOG.warn("Query param 'page' is not valid integer", ex);
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
        String pathId = ctx.path("id").value();

        long replayId;
        try {
            replayId = Long.parseUnsignedLong(pathId);
        } catch (NumberFormatException ex) {
            LOG.warn("Path param 'id' is not a valid long", ex);
            throw new BadRequestException(ERROR_INVALID_REQUEST);
        }

        ReplayEntity entity = replayDao.getReplay(replayId);

        return ReplayView.createHeader(entity);
    }

    public String getReplayMoveList(Context ctx) {
        String pathId = ctx.path("id").value();

        long replayId;
        try {
            replayId = Long.parseUnsignedLong(pathId);
        } catch (NumberFormatException ex) {
            LOG.warn("Path param 'id' is not a valid long", ex);
            throw new BadRequestException(ERROR_INVALID_REQUEST);
        }

        String moveListJson = replayDao.getReplayMoveList(replayId);

//        ctx.setResponseHeader("Cache-Control", LONG_CACHE_CONTROL);
        ctx.setResponseType(MediaType.JSON);
        return moveListJson;
    }

    public List<ChallengeView> getChallengeList(Context ctx) {
        String participants = ctx.query("participants").value("");

        PlayerState player = authService.getSessionPlayer(ctx);

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
        Optional<String> afterIdPath = ctx.query("afterId").toOptional();
        long userId = ctx.query("userId").longValue();

        Long afterId;
        try {
            afterId = afterIdPath.map(Long::parseUnsignedLong).orElse(null);
        } catch (NumberFormatException ex) {
            LOG.warn("Query param 'afterId' must be a valid integer", ex);
            throw new BadRequestException(ERROR_INVALID_REQUEST);
        }

        List<ReplayEntity> entityList = replayDao.getUserReplays(userId, afterId, 25);
        if (entityList.isEmpty()) {
            return List.of();
        }

        return entityList.stream().map(ReplayView::createRow).toList();
    }

    public record ChessRoomResp(List<ChessView> chessList, List<ChessView> selfChessList) {}

    public ChessRoomResp getRoomLists(Context ctx) {
        Optional<String> pageQuery = ctx.query("page").toOptional();
        Optional<String> countQuery = ctx.query("count").toOptional();

        int page;
        int count;
        try {
            page = pageQuery.map(Integer::parseUnsignedInt).orElse(1);
            count = countQuery.map(Integer::parseUnsignedInt).orElse(PER_PAGE);
        } catch (NumberFormatException ex) {
            LOG.warn("Query param 'page' and 'count' must be a valid integer", ex);
            throw new BadRequestException(ERROR_INVALID_REQUEST);
        }

        PlayerState player = authService.getOptionalSessionPlayer(ctx);
        if (player == null) {
            LOG.info("Player is not provided when requesting rooms list, defaulting to empty list");
        }

        List<ChessView> viewList = dictionaryDao.getChessViews(page, count);
        List<ChessView> selfViewList = player != null ? dictionaryDao.getUserChessViews(player.getId()) : List.of();

        return new ChessRoomResp(viewList, selfViewList);
    }
}