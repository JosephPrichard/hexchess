package web.controllers;

import io.jooby.Context;
import models.entities.ChallengeEntity;
import models.views.ServiceView;
import models.views.SessionView;
import services.daos.DictionaryDao;
import services.daos.ChallengeDao;
import io.jooby.Cookie;
import io.jooby.exception.StatusCodeException;
import io.jooby.test.MockContext;
import io.jooby.test.MockResponse;
import io.jooby.test.MockRouter;
import io.jooby.test.MockValue;
import models.enums.ColorSelect;
import models.state.PlayerState;
import models.enums.TimeControl;
import models.entities.UserEntity;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.Test;
import services.game.GameService;
import services.daos.UserDao;
import web.reusable.AuthService;
import web.State;

import java.util.concurrent.atomic.AtomicReference;

import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

public class FormControllerTest {

    @Test
    public void testRegister() throws UserDao.TakenUsernameException {
        // given
        UserEntity user = new UserEntity(1L, "testUser", "USA");
        PlayerState player = new PlayerState(1L, "testUser", "USA", 0f);
        Cookie cookie = new Cookie("sessionToken");

        UserDao mockUserDao = mock(UserDao.class);
        AuthService mockAuthService = mock(AuthService.class);

        State state = new State();
        state.setUserDao(mockUserDao);
        state.setAuthService(mockAuthService);

        DictionaryDao mockDict = mock(DictionaryDao.class);
        state.setDictionaryDao(mockDict);

        MockRouter mockRouter = new MockRouter(new FormController(state));

        MockContext mockContext = new MockContext();
        mockContext.setBodyObject(new FormController.RegisterBody("testUser", "testPassword", "testPassword"));

        // when
        when(mockUserDao.insert(any(), any())).thenReturn(user);
        when(mockAuthService.createSessionId()).thenReturn("sessionToken");
        when(mockAuthService.createSessionCookie(any())).thenReturn(cookie);

        AtomicReference<MockResponse> response = new AtomicReference<>();
        MockValue value = mockRouter.post("/forms/register", mockContext, response::set);
        Object actualCookie = response.get().getHeaders().get("Set-Cookie");

        // then
        verify(mockUserDao, times(1)).insert("testUser", "testPassword");
        verify(mockDict, times(1)).setSession(anyString(), eq(player), anyLong());
        verify(mockAuthService, times(1)).createSessionCookie("sessionToken");

        Assertions.assertEquals(actualCookie, cookie.toString());
        Assertions.assertEquals(new SessionView(1L, "testUser", "USA", 0f, null), value.value());
    }

    @Test
    public void testLogin() {
        // given
        PlayerState player = new PlayerState(1L, "testUser", "us", 0f);
        String country = "us";
        Cookie cookie = new Cookie("sessionToken");

        UserDao mockUserDao = mock(UserDao.class);
        AuthService mockAuthService = mock(AuthService.class);
        DictionaryDao mockDict = mock(DictionaryDao.class);

        State state = new State();
        state.setUserDao(mockUserDao);
        state.setAuthService(mockAuthService);
        state.setDictionaryDao(mockDict);

        MockRouter mockRouter = new MockRouter(new FormController(state));

        MockContext mockContext = new MockContext();
        mockContext.setBodyObject(new FormController.LoginBody("testUser", "testPassword"));

        // when
        when(mockUserDao.verify(any(), any())).thenReturn(new UserDao.VerifiedUser(1L, "testUser", country, 0f));
        when(mockAuthService.createSessionId()).thenReturn("sessionToken");
        when(mockAuthService.createSessionCookie(any())).thenReturn(cookie);

        AtomicReference<MockResponse> resp = new AtomicReference<>();
        MockValue value = mockRouter.post("/forms/login", mockContext, resp::set);
        Object actualCookie = resp.get().getHeaders().get("Set-Cookie");

        // then
        verify(mockUserDao, times(1)).verify("testUser", "testPassword");
        verify(mockDict, times(1)).setSession(anyString(), eq(player), anyLong());
        verify(mockAuthService, times(1)).createSessionCookie("sessionToken");

        Assertions.assertEquals(actualCookie, cookie.toString());
        Assertions.assertEquals(new SessionView(1L, "testUser", "us", 0f, null), value.value());
    }

    @Test
    public void testCreateGame() {
        // given
        GameService mockGameService = mock(GameService.class);

        State state = new State();
        state.setGameService(mockGameService);

        MockRouter mockRouter = new MockRouter(new FormController(state));

        MockContext mockContext = new MockContext();
        mockContext.setBodyObject(new FormController.CreateGameBody(null, null));

        // when
        when(mockGameService.create(any(), any())).thenReturn("test-id");

        MockValue result = mockRouter.post("/forms/games/create", mockContext);

        // then
        verify(mockGameService, times(1)).create(ColorSelect.RANDOM, TimeControl.UNLIMITED);

        Assertions.assertEquals(new FormController.CreateGameResp("test-id"), result.value());
    }

    private MockContext mockChallengeCtx(long challengerId, long challengeeId, FormController.UpdtAction action) {
        MockContext mockContext = new MockContext();
        mockContext.setBodyObject(new FormController.UpdateChallengeBody(challengeeId, challengerId, action));
        return mockContext;
    }

    @Test
    public void testAcceptChallenge() {
        // given
        long challengeeId = 1L;
        long challengerId = 2L;

        ChallengeDao mockChallengeDao = mock(ChallengeDao.class);
        GameService mockGameService = mock(GameService.class);
        DictionaryDao mockDictionaryDao = mock(DictionaryDao.class);
        AuthService mockAuthService = mock(AuthService.class);

        State state = new State();
        state.setChallengeDao(mockChallengeDao);
        state.setGameService(mockGameService);
        state.setDictionaryDao(mockDictionaryDao);
        state.setAuthService(mockAuthService);

        MockRouter mockRouter = new MockRouter(new FormController(state));
        MockContext mockContext = mockChallengeCtx(challengerId, challengeeId, FormController.UpdtAction.ACCEPT);

        // when
        when(mockAuthService.getSessionPlayer(any())).thenReturn(new PlayerState(challengeeId, "playerName", "us", 0f));
        when(mockGameService.create(any(), any())).thenReturn("test-id");
        when(mockChallengeDao.delete(anyLong(), anyLong())).thenReturn(new ChallengeDao.DeleteResult(1L, 2L, "UNLIMITED", "WHITE"));

        MockValue value = mockRouter.post("/forms/challenges/update", mockContext);

        // then
        verify(mockChallengeDao, times(1)).delete(challengerId, challengeeId);
        verify(mockGameService, times(1)).create(ColorSelect.WHITE, TimeControl.UNLIMITED);

        Assertions.assertEquals(new FormController.UpdateChallengeResp("test-id"), value.value());
    }

    @Test
    public void testNoAcceptChallenge() {
        // given
        long challengeeId = 1L;
        long challengerId = 2L;

        ChallengeDao mockChallengeDao = mock(ChallengeDao.class);
        GameService mockGameService = mock(GameService.class);
        DictionaryDao mockDictionaryDao = mock(DictionaryDao.class);
        AuthService mockAuthService = mock(AuthService.class);

        State state = new State();
        state.setChallengeDao(mockChallengeDao);
        state.setGameService(mockGameService);
        state.setDictionaryDao(mockDictionaryDao);
        state.setAuthService(mockAuthService);

        MockRouter mockRouter = new MockRouter(new FormController(state));
        MockContext mockContext = mockChallengeCtx(challengerId, challengeeId, FormController.UpdtAction.ACCEPT);

        // when
        when(mockAuthService.getSessionPlayer(any())).thenReturn(new PlayerState(challengeeId, "playerName", "us", 0f));
        when(mockChallengeDao.delete(anyLong(), anyLong())).thenReturn(null);

        Assertions.assertThrows(StatusCodeException.class, () -> mockRouter.post("/forms/challenges/update", mockContext));

        // then
        verify(mockChallengeDao, times(1)).delete(challengerId, challengeeId);
        verify(mockGameService, times(0)).create(any(), any());
    }

    @Test
    public void testDeleteChallenge() {
        // given
        long challengeeId = 1L;
        long challengerId = 2L;

        ChallengeDao mockChallengeDao = mock(ChallengeDao.class);
        GameService mockGameService = mock(GameService.class);
        DictionaryDao mockDictionaryDao = mock(DictionaryDao.class);
        AuthService mockAuthService = mock(AuthService.class);

        State state = new State();
        state.setChallengeDao(mockChallengeDao);
        state.setGameService(mockGameService);
        state.setDictionaryDao(mockDictionaryDao);
        state.setAuthService(mockAuthService);

        MockRouter mockRouter = new MockRouter(new FormController(state));
        MockContext mockContext = mockChallengeCtx(challengerId, challengeeId, FormController.UpdtAction.DELETE);

        // when
        when(mockAuthService.getSessionPlayer(any())).thenReturn(new PlayerState(challengerId, "playerName", "us", 0f));
        when(mockChallengeDao.delete(anyLong(), anyLong())).thenReturn(new ChallengeDao.DeleteResult(1L, 2L, "UNLIMITED", "WHITE"));

        MockValue value = mockRouter.post("/forms/challenges/update", mockContext);

        // then
        verify(mockChallengeDao, times(1)).delete(challengerId, challengeeId);
        verify(mockGameService, times(0)).create(any(), any());

        Assertions.assertEquals(new FormController.UpdateChallengeResp(null), value.value());
    }

    @Test
    public void testRejectChallenge() {
        // given
        long challengeeId = 1L;
        long challengerId = 2L;

        ChallengeDao mockChallengeDao = mock(ChallengeDao.class);
        GameService mockGameService = mock(GameService.class);
        DictionaryDao mockDictionaryDao = mock(DictionaryDao.class);
        AuthService mockAuthService = mock(AuthService.class);

        State state = new State();
        state.setChallengeDao(mockChallengeDao);
        state.setGameService(mockGameService);
        state.setDictionaryDao(mockDictionaryDao);
        state.setAuthService(mockAuthService);

        MockRouter mockRouter = new MockRouter(new FormController(state));
        MockContext mockContext = mockChallengeCtx(challengerId, challengeeId, FormController.UpdtAction.REJECT);

        // when
        when(mockAuthService.getSessionPlayer(any())).thenReturn(new PlayerState(challengeeId, "playerName", "us", 0f));
        when(mockChallengeDao.delete(anyLong(), anyLong())).thenReturn(new ChallengeDao.DeleteResult(1L, 2L, "UNLIMITED", "WHITE"));

        MockValue value = mockRouter.post("/forms/challenges/update", mockContext);

        // then
        verify(mockChallengeDao, times(1)).delete(challengerId, challengeeId);
        verify(mockGameService, times(0)).create(any(), any());

        Assertions.assertEquals(new FormController.UpdateChallengeResp(null), value.value());
    }

    public static ChallengeEntity challengeFromIds(long challengerId, long challengeeId) {
        ChallengeEntity entity = new ChallengeEntity();
        entity.setChallengerId(challengerId);
        entity.setChallengeeId(challengeeId);
        return entity;
    }

    @Test
    public void testCreateChallenge() {
        // given
        long challengeeId = 1L;
        long challengerId = 2L;

        ChallengeDao mockChallengeDao = mock(ChallengeDao.class);
        DictionaryDao mockDictionaryDao = mock(DictionaryDao.class);
        AuthService mockAuthService = mock(AuthService.class);

        State state = new State();
        state.setChallengeDao(mockChallengeDao);
        state.setDictionaryDao(mockDictionaryDao);
        state.setAuthService(mockAuthService);

        // using a spy instead of a mock router so we can do a partial mock
        FormController sut = spy(new FormController(state));

        Context mockContext = mock(Context.class);

        // when
        when(mockContext.body(any())).thenReturn(new FormController.CreateChallengeBody(challengeeId, TimeControl.REAL_TIME, ColorSelect.RANDOM));

        doNothing().when(sut).dispatchBroadcastChallenge(any());
        doNothing().when(sut).dispatchDeleteExpired(anyLong());

        when(mockAuthService.getSessionPlayer(any())).thenReturn(new PlayerState(challengerId, "playerName", "us", 0f));
        when(mockChallengeDao.insert(anyLong(), anyLong(), anyString(), anyString())).thenReturn(challengeFromIds(challengeeId, challengerId));

        ServiceView response = sut.createChallenge(mockContext);

        // then
        verify(mockChallengeDao, times(1)).insert(challengerId, challengeeId, "REAL_TIME", "RANDOM");
        verify(sut, times(1)).dispatchBroadcastChallenge(challengeFromIds(challengeeId, challengerId));
        verify(sut, times(1)).dispatchDeleteExpired(challengerId);

        Assertions.assertEquals(ServiceView.SUCCESS, response);
    }
}
