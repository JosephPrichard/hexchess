package web.controllers;

import services.daos.DictionaryDao;
import services.daos.ChallengeDao;
import io.jooby.Cookie;
import io.jooby.exception.StatusCodeException;
import io.jooby.test.MockContext;
import io.jooby.test.MockResponse;
import io.jooby.test.MockRouter;
import io.jooby.test.MockValue;
import models.common.ColorSelect;
import models.entities.PlayerEntity;
import models.common.TimeControl;
import models.entities.UserEntity;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.Test;
import services.game.GameService;
import services.daos.UserDao;
import web.reusable.SessionService;
import web.WebConstants;
import web.State;

import java.util.concurrent.atomic.AtomicReference;

import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

public class FormControllerTest {

    @Test
    public void testPostRegister() throws UserDao.TakenUsernameException {
        // given
        UserEntity user = new UserEntity(1L, "testUser", "USA");
        PlayerEntity player = new PlayerEntity(1L, "testUser", "us", 0f);
        Cookie cookie = new Cookie("sessionToken");

        UserDao mockUserDao = mock(UserDao.class);
        SessionService mockSessionService = mock(SessionService.class);

        when(mockUserDao.insert(any(), any())).thenReturn(user);
        when(mockSessionService.createId()).thenReturn("sessionToken");
        when(mockSessionService.createCookie(any(), anyLong(), any(), any())).thenReturn(cookie);

        State state = new State();
        state.setUserDao(mockUserDao);
        state.setSessionService(mockSessionService);

        DictionaryDao mockDict = mock(DictionaryDao.class);
        state.setDictionaryDao(mockDict);

        MockRouter mockRouter = new MockRouter(new FormController(state));

        MockContext mockContext = new MockContext();
        mockContext.setBodyObject(new FormController.RegisterBody("testUser", "testPassword", "testPassword"));

        // when
        AtomicReference<MockResponse> response = new AtomicReference<>();
        MockValue value = mockRouter.post("/forms/register", mockContext, response::set);
        Object actualCookie = response.get().getHeaders().get("Set-Cookie");

        // then
        verify(mockUserDao, times(1)).insert("testUser", "testPassword");
        verify(mockDict, times(1)).setSession(anyString(), eq(player), anyLong());
        verify(mockSessionService, times(1)).createCookie(eq("sessionToken"), eq(player.id), eq(player.name), any());

        Assertions.assertEquals(actualCookie, cookie.toString());
        Assertions.assertEquals(WebConstants.SUCCESS_REGISTER, value.value());
    }

    @Test
    public void testPostLogin() {
        // given
        PlayerEntity player = new PlayerEntity(1L, "testUser", "us", 0f);
        String country = "us";
        Cookie cookie = new Cookie("sessionToken");

        UserDao mockUserDao = mock(UserDao.class);
        SessionService mockSessionService = mock(SessionService.class);

        when(mockUserDao.verify(any(), any())).thenReturn(new UserDao.VerifiedUser(1L, "testUser", country, 0f));
        when(mockSessionService.createId()).thenReturn("sessionToken");
        when(mockSessionService.createCookie(any(), anyLong(), any(), any())).thenReturn(cookie);

        State state = new State();
        state.setUserDao(mockUserDao);
        state.setSessionService(mockSessionService);

        DictionaryDao mockDict = mock(DictionaryDao.class);
        state.setDictionaryDao(mockDict);

        MockRouter mockRouter = new MockRouter(new FormController(state));

        MockContext mockContext = new MockContext();
        mockContext.setBodyObject(new FormController.LoginBody("testUser", "testPassword"));

        // when
        AtomicReference<MockResponse> resp = new AtomicReference<>();
        MockValue value = mockRouter.post("/forms/login", mockContext, resp::set);
        Object actualCookie = resp.get().getHeaders().get("Set-Cookie");

        // then
        verify(mockUserDao, times(1)).verify("testUser", "testPassword");
        verify(mockDict, times(1)).setSession(anyString(), eq(player), anyLong());
        verify(mockSessionService, times(1)).createCookie(eq("sessionToken"), eq(player.id), eq(player.name), any());

        Assertions.assertEquals(actualCookie, cookie.toString());
        Assertions.assertEquals(WebConstants.SUCCESS_LOGIN, value.value());
    }

    @Test
    public void testPostCreateGame() {
        // given
        GameService mockGameService = mock(GameService.class);
        when(mockGameService.create(any(), any())).thenReturn("test-id");

        State state = new State();
        state.setGameService(mockGameService);

        MockRouter mockRouter = new MockRouter(new FormController(state));

        MockContext mockContext = new MockContext();
        mockContext.setBodyObject(new FormController.CreateGameBody(null, null));

        // when
        MockValue result = mockRouter.post("/forms/games/create", mockContext);

        // then
        verify(mockGameService, times(1)).create(ColorSelect.RANDOM, TimeControl.UNLIMITED);
        Assertions.assertEquals(new FormController.CreateGameResp(WebConstants.SUCCESS_GENERIC, "test-id"), result.value());
    }

    private MockContext mockChallengeCtx(long challengerId, long challengeeId, String action) {
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
        SessionService mockSessionService = mock(SessionService.class);

        when(mockSessionService.parseSession(any())).thenReturn(new SessionService.SessionValue("sessionId", challengeeId, "username", "us"));
        when(mockDictionaryDao.getSession("sessionId")).thenReturn(new PlayerEntity(challengeeId, "playerName", "us", 0f));
        when(mockGameService.create(any(), any())).thenReturn("test-id");
        when(mockChallengeDao.delete(anyLong(), anyLong())).thenReturn(new ChallengeDao.DeleteResult(1L, 2L, "UNLIMITED", "WHITE"));

        State state = new State();
        state.setChallengeDao(mockChallengeDao);
        state.setGameService(mockGameService);
        state.setDictionaryDao(mockDictionaryDao);
        state.setSessionService(mockSessionService);

        MockRouter mockRouter = new MockRouter(new FormController(state));
        MockContext mockContext = mockChallengeCtx(challengerId, challengeeId, "ACCEPT");

        // when
        MockValue value = mockRouter.post("/forms/challenges/update", mockContext);

        // then
        verify(mockChallengeDao, times(1)).delete(challengerId, challengeeId);
        verify(mockGameService, times(1)).create(ColorSelect.WHITE, TimeControl.UNLIMITED);

        Assertions.assertEquals(new FormController.UpdateChallengeResp(WebConstants.SUCCESS_UPDATE_CHALLENGE, "test-id"), value.value());
    }

    @Test
    public void testNoAcceptChallenge() {
        // given
        long challengeeId = 1L;
        long challengerId = 2L;

        ChallengeDao mockChallengeDao = mock(ChallengeDao.class);
        GameService mockGameService = mock(GameService.class);
        DictionaryDao mockDictionaryDao = mock(DictionaryDao.class);
        SessionService mockSessionService = mock(SessionService.class);

        when(mockSessionService.parseSession(any())).thenReturn(new SessionService.SessionValue("sessionId", challengeeId, "username", "us"));
        when(mockDictionaryDao.getSession("sessionId")).thenReturn(new PlayerEntity(challengeeId, "playerName", "us", 0f));
        when(mockChallengeDao.delete(anyLong(), anyLong())).thenReturn(null);

        State state = new State();
        state.setChallengeDao(mockChallengeDao);
        state.setGameService(mockGameService);
        state.setDictionaryDao(mockDictionaryDao);
        state.setSessionService(mockSessionService);

        MockRouter mockRouter = new MockRouter(new FormController(state));
        MockContext mockContext = mockChallengeCtx(challengerId, challengeeId, "ACCEPT");

        // when
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
        SessionService mockSessionService = mock(SessionService.class);

        when(mockSessionService.parseSession(any())).thenReturn(new SessionService.SessionValue("sessionId", challengerId, "username", "us"));
        when(mockDictionaryDao.getSession("sessionId")).thenReturn(new PlayerEntity(challengerId, "playerName", "us", 0f));
        when(mockChallengeDao.delete(anyLong(), anyLong())).thenReturn(new ChallengeDao.DeleteResult(1L, 2L, "UNLIMITED", "WHITE"));

        State state = new State();
        state.setChallengeDao(mockChallengeDao);
        state.setGameService(mockGameService);
        state.setDictionaryDao(mockDictionaryDao);
        state.setSessionService(mockSessionService);

        MockRouter mockRouter = new MockRouter(new FormController(state));
        MockContext mockContext = mockChallengeCtx(challengerId, challengeeId, "DELETE");

        // when
        MockValue value = mockRouter.post("/forms/challenges/update", mockContext);

        // then
        verify(mockChallengeDao, times(1)).delete(challengerId, challengeeId);
        verify(mockGameService, times(0)).create(any(), any());

        Assertions.assertEquals(new FormController.UpdateChallengeResp(WebConstants.SUCCESS_UPDATE_CHALLENGE, null), value.value());
    }

    @Test
    public void testRejectChallenge() {
        // given
        long challengeeId = 1L;
        long challengerId = 2L;

        ChallengeDao mockChallengeDao = mock(ChallengeDao.class);
        GameService mockGameService = mock(GameService.class);
        DictionaryDao mockDictionaryDao = mock(DictionaryDao.class);
        SessionService mockSessionService = mock(SessionService.class);

        when(mockSessionService.parseSession(any())).thenReturn(new SessionService.SessionValue("sessionId", challengeeId, "username", "us"));
        when(mockDictionaryDao.getSession("sessionId")).thenReturn(new PlayerEntity(challengeeId, "playerName", "us", 0f));
        when(mockChallengeDao.delete(anyLong(), anyLong())).thenReturn(new ChallengeDao.DeleteResult(1L, 2L, "UNLIMITED", "WHITE"));

        State state = new State();
        state.setChallengeDao(mockChallengeDao);
        state.setGameService(mockGameService);
        state.setDictionaryDao(mockDictionaryDao);
        state.setSessionService(mockSessionService);

        MockRouter mockRouter = new MockRouter(new FormController(state));
        MockContext mockContext = mockChallengeCtx(challengerId, challengeeId, "REJECT");

        // when
        MockValue value = mockRouter.post("/forms/challenges/update", mockContext);

        // then
        verify(mockChallengeDao, times(1)).delete(challengerId, challengeeId);
        verify(mockGameService, times(0)).create(any(), any());

        Assertions.assertEquals(new FormController.UpdateChallengeResp(WebConstants.SUCCESS_UPDATE_CHALLENGE, null), value.value());
    }
}
