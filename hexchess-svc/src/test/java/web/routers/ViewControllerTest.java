package web.routers;

import io.jooby.Context;
import io.jooby.exception.BadRequestException;
import io.jooby.internal.SingleValue;
import io.jooby.test.MockContext;
import io.jooby.test.MockRouter;
import io.jooby.test.MockValue;
import models.common.ColorSelect;
import models.common.TimeControl;
import models.entities.RankedEntity;
import models.entities.ReplayEntity;
import models.entities.UserEntity;
import models.views.ChessView;
import models.views.ReplayView;
import models.views.UserView;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.Test;
import services.daos.DictionaryDao;
import services.daos.UserDao;
import web.State;
import web.controllers.ViewController;
import web.reusable.AuthService;

import java.util.Arrays;
import java.util.List;
import java.util.concurrent.CompletableFuture;

import static org.mockito.Mockito.*;
import static web.controllers.ViewController.PER_PAGE;

public class ViewControllerTest {

    private static UserEntity createUserEntity(long id) {
        UserEntity user = new UserEntity();
        user.setId(id);
        return user;
    }

    private static UserView createUserView(long id, int rank) {
        UserView user = new UserView();
        user.setId(id);
        user.setRank(rank);
        user.setJoinedOn("");
        return user;
    }

    @Test
    public void testGetLeaderboard() {
        // given
        DictionaryDao dictionaryDao = mock(DictionaryDao.class);
        UserDao userDao = mock(UserDao.class);

        State state = new State();
        state.setDictionaryDao(dictionaryDao);
        state.setUserDao(userDao);

        List<RankedEntity> rankedEntities = Arrays.asList(new RankedEntity(1, 1), new RankedEntity(2, 2));
        List<UserEntity> entityList = Arrays.asList(createUserEntity(1), createUserEntity(2));

        MockRouter mockRouter = new MockRouter(new ViewController(state));

        // when
        when(dictionaryDao.getLeaderboardPage(anyInt(), anyInt())).thenReturn(new DictionaryDao.Leaderboard(rankedEntities, 1));
        when(userDao.getByRankedUsers(any())).thenReturn(entityList);

        MockValue value = mockRouter.get("/views/leaderboard", new MockContext());

        // then
        verify(dictionaryDao).getLeaderboardPage(1, PER_PAGE);
        verify(userDao).getByRankedUsers(rankedEntities);

        List<UserView> viewList = List.of(createUserView(1, 1), createUserView(2, 2));

        Assertions.assertEquals(new ViewController.LeaderboardResp(1, viewList), value.value());
    }

    private static ReplayEntity createReplayEntity(long id) {
        ReplayEntity replay = new ReplayEntity();
        replay.setId(id);
        return replay;
    }

    private static ReplayView createReplayView(long id) {
        ReplayView replay = new ReplayView();
        replay.setId(id);
        return replay;
    }

    @Test
    public void testGetPlayer() throws Exception {
        // given
        DictionaryDao dictionaryDao = mock(DictionaryDao.class);

        State state = new State();
        state.setDictionaryDao(dictionaryDao);

        // using a spy instead of a mock router so we can do a partial mock
        ViewController sut = spy(new ViewController(state));

        Context mockContext = mock(Context.class);

        UserEntity user = createUserEntity(1);
        List<ReplayEntity> entityList = List.of(createReplayEntity(1));

        // when
        when(mockContext.path("id")).thenReturn(new SingleValue(mockContext, "id", "1"));

        when(sut.dispatchGetById(anyLong())).thenReturn(CompletableFuture.completedFuture(user));
        when(sut.dispatchUserReplays(anyLong())).thenReturn(CompletableFuture.completedFuture(entityList));
        when(dictionaryDao.getLeaderboardRank(anyLong())).thenReturn(5);

        ViewController.UserWithReplaysResp response = sut.getPlayer(mockContext);

        // then
        verify(dictionaryDao).getLeaderboardRank(1);
        verify(sut).dispatchGetById(1);
        verify(sut).dispatchUserReplays(1);

        UserView userView = createUserView(1, 5);
        List<ReplayView> viewList = List.of(createReplayView(1));

        Assertions.assertEquals(new ViewController.UserWithReplaysResp(userView, viewList), response);
    }

    @Test
    public void testGetPlayer_NoPlayer_Throws() {
        // given
        DictionaryDao dictionaryDao = mock(DictionaryDao.class);

        State state = new State();
        state.setDictionaryDao(dictionaryDao);

        // using a spy instead of a mock router so we can do a partial mock
        ViewController sut = spy(new ViewController(state));

        Context mockContext = mock(Context.class);

        // when
        when(mockContext.path("id")).thenReturn(new SingleValue(mockContext, "id", "1"));

        when(sut.dispatchGetById(anyLong())).thenReturn(CompletableFuture.completedFuture(null));
        when(sut.dispatchUserReplays(anyLong())).thenReturn(CompletableFuture.completedFuture(List.of()));

        // then
        Assertions.assertThrows(BadRequestException.class, () -> sut.getPlayer(mockContext));
    }

    @Test
    public void testGetRoomLists() {
        // given
        DictionaryDao dictionaryDao = mock(DictionaryDao.class);
        AuthService authService = mock(AuthService.class);

        State state = new State();
        state.setDictionaryDao(dictionaryDao);
        state.setAuthService(authService);

        List<ChessView> viewList = List.of(new ChessView("abc123", null, null, false, ColorSelect.RANDOM, TimeControl.UNLIMITED));

        MockRouter mockRouter = new MockRouter(new ViewController(state));

        MockContext mockContext = new MockContext();
        mockContext.setQueryString("?page=1&count=20");

        // when
        when(dictionaryDao.getChessViews(anyInt(), anyInt())).thenReturn(viewList);
        when(authService.getOptionalSessionPlayer(any())).thenReturn(null);

        MockValue value = mockRouter.get("/views/chess/rooms", mockContext);

        // then
        verify(dictionaryDao).getChessViews(1, 20);

        Assertions.assertEquals(new ViewController.ChessRoomResp(viewList, List.of()), value.value());
    }
}
