package web.controllers;

import io.jooby.Context;
import io.jooby.exception.BadRequestException;
import io.jooby.internal.SingleValue;
import io.jooby.test.MockContext;
import io.jooby.test.MockRouter;
import io.jooby.test.MockValue;
import models.common.ColorSelect;
import models.common.TimeControl;
import models.entities.UserRankEntity;
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
import web.reusable.AuthService;

import java.util.Arrays;
import java.util.List;
import java.util.concurrent.CompletableFuture;

import static mocks.UserMocks.*;
import static mocks.ReplayMocks.*;
import static org.mockito.Mockito.*;
import static web.controllers.ViewController.PER_PAGE;

public class ViewControllerTest {

    @Test
    public void testGetLeaderboard() {
        // given
        DictionaryDao dictionaryDao = mock(DictionaryDao.class);
        UserDao userDao = mock(UserDao.class);

        State state = new State();
        state.setDictionaryDao(dictionaryDao);
        state.setUserDao(userDao);

        List<UserRankEntity> rankedEntities = Arrays.asList(new UserRankEntity(1, 1), new UserRankEntity(2, 2));
        List<UserEntity> entityList = Arrays.asList(USER_ENTITY1, USER_ENTITY2);

        MockRouter mockRouter = new MockRouter(new ViewController(state));

        // when
        when(dictionaryDao.getLeaderboardPage(anyInt(), anyInt())).thenReturn(new DictionaryDao.Leaderboard(rankedEntities, 1));
        when(userDao.getByRankedUsers(any())).thenReturn(entityList);

        MockValue value = mockRouter.get("/views/leaderboard", new MockContext());

        // then
        verify(dictionaryDao).getLeaderboardPage(1, PER_PAGE);
        verify(userDao).getByRankedUsers(rankedEntities);

        List<UserView> viewList = List.of(USER_VIEW1, USER_VIEW2);

        Assertions.assertEquals(new ViewController.LeaderboardResp(1, viewList), value.value());
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

        List<ReplayEntity> entityList = List.of(REPLAY_ENTITY1);

        // when
        when(mockContext.path("id")).thenReturn(new SingleValue(mockContext, "id", "1"));

        when(sut.dispatchGetById(anyLong())).thenReturn(CompletableFuture.completedFuture(USER_ENTITY1));
        when(sut.dispatchUserReplays(anyLong())).thenReturn(CompletableFuture.completedFuture(entityList));
        when(dictionaryDao.getLeaderboardRank(anyLong())).thenReturn(1);

        ViewController.UserWithReplaysResp response = sut.getPlayer(mockContext);

        // then
        verify(dictionaryDao).getLeaderboardRank(1);
        verify(sut).dispatchGetById(1);
        verify(sut).dispatchUserReplays(1);

        List<ReplayView> viewList = List.of(REPLAY_VIEW1);

        Assertions.assertEquals(new ViewController.UserWithReplaysResp(USER_VIEW1, viewList), response);
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
