package web.controllers;

import daos.ChallengeDao;
import io.jooby.Cookie;
import io.jooby.Formdata;
import io.jooby.exception.StatusCodeException;
import io.jooby.test.MockContext;
import io.jooby.test.MockResponse;
import io.jooby.test.MockRouter;
import io.jooby.test.MockValue;
import models.PlayerEntity;
import models.UserEntity;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.Test;
import services.GameService;
import services.RemoteDict;
import daos.UserDao;
import services.SessionService;
import web.State;

import java.util.concurrent.atomic.AtomicReference;

import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

public class FormControllerTest {

    @Test
    public void testPostRegister() throws UserDao.TakenUsernameException {
        // given
        UserEntity user = new UserEntity(1L, "testUser", "USA");
        PlayerEntity player = new PlayerEntity(1L, "testUser");
        Cookie cookie = new Cookie("sessionToken");

        UserDao mockUserDao = mock(UserDao.class);
        SessionService mockSessionService = mock(SessionService.class);

        when(mockUserDao.insert(any(), any())).thenReturn(user);
        when(mockSessionService.createId()).thenReturn("sessionToken");
        when(mockSessionService.createCookie(any(), anyLong(), any(), any())).thenReturn(cookie);

        State state = new State();
        state.setUserDao(mockUserDao);
        state.setSessionService(mockSessionService);

        RemoteDict mockDict = mock(RemoteDict.class);
        state.setRemoteDict(mockDict);

        MockRouter mockRouter = new MockRouter(new FormController(state));

        MockContext mockContext = new MockContext();
        Formdata mockForm = Formdata.create(mockContext);
        mockForm.put("username", "testUser");
        mockForm.put("password", "testPassword");
        mockForm.put("duplicate-password", "testPassword");
        mockContext.setForm(mockForm);

        // when
        AtomicReference<MockResponse> response = new AtomicReference<>();
        mockRouter.post("/forms/register", mockContext, response::set);
        Object actualCookie = response.get().getHeaders().get("Set-Cookie");

        // then
        verify(mockUserDao, times(1)).insert("testUser", "testPassword");
        verify(mockDict, times(1)).setSession(anyString(), eq(player), anyLong());
        verify(mockSessionService, times(1)).createCookie(eq("sessionToken"), eq(player.getId()), eq(player.getName()), any());

        Assertions.assertEquals(actualCookie, cookie.toString());
    }

    @Test
    public void testPostLogin() {
        // given
        PlayerEntity player = new PlayerEntity(1L, "testUser");
        String country = "us";
        Cookie cookie = new Cookie("sessionToken");

        UserDao mockUserDao = mock(UserDao.class);
        SessionService mockSessionService = mock(SessionService.class);

        when(mockUserDao.verify(any(), any())).thenReturn(new UserDao.VerifiedUser(1L, "testUser", country));
        when(mockSessionService.createId()).thenReturn("sessionToken");
        when(mockSessionService.createCookie(any(), anyLong(), any(), any())).thenReturn(cookie);

        State state = new State();
        state.setUserDao(mockUserDao);
        state.setSessionService(mockSessionService);

        RemoteDict mockDict = mock(RemoteDict.class);
        state.setRemoteDict(mockDict);

        MockRouter mockRouter = new MockRouter(new FormController(state));

        MockContext mockContext = new MockContext();
        Formdata mockForm = Formdata.create(mockContext);
        mockForm.put("username", "testUser");
        mockForm.put("password", "testPass");
        mockContext.setForm(mockForm);

        // when
        AtomicReference<MockResponse> resp = new AtomicReference<>();
        mockRouter.post("/forms/login", mockContext, resp::set);
        Object actualCookie = resp.get().getHeaders().get("Set-Cookie");

        // then
        verify(mockUserDao, times(1)).verify("testUser", "testPass");
        verify(mockDict, times(1)).setSession(anyString(), eq(player), anyLong());
        verify(mockSessionService, times(1)).createCookie(eq("sessionToken"), eq(player.getId()), eq(player.getName()), any());

        Assertions.assertEquals(actualCookie, cookie.toString());
    }

    @Test
    public void testPostCreateGame() {
        // given
        GameService mockGameService = mock(GameService.class);
        when(mockGameService.create(null)).thenReturn("test-id");

        State state = new State();
        state.setGameService(mockGameService);

        MockRouter mockRouter = new MockRouter(new FormController(state));

        // when
        MockValue result = mockRouter.post("/forms/game");

        // then
        verify(mockGameService, times(1)).create(null);
        Assertions.assertEquals("test-id", result.value());
    }

    private MockContext mockChallengeCtx(long challengerId, long challengeeId, String action) {
        MockContext mockContext = new MockContext();
        Formdata mockForm = Formdata.create(mockContext);
        mockForm.put("challengeeId", Long.toString(challengeeId));
        mockForm.put("challengerId",  Long.toString(challengerId));
        mockForm.put("action", action);
        mockContext.setForm(mockForm);
        return mockContext;
    }

    @Test
    public void testAcceptChallenge() {
        // given
        long challengeeId = 1L;
        long challengerId = 2L;

        ChallengeDao mockChallengeDao = mock(ChallengeDao.class);
        GameService mockGameService = mock(GameService.class);
        RemoteDict mockRemoteDict = mock(RemoteDict.class);
        SessionService mockSessionService = mock(SessionService.class);

        when(mockSessionService.parseSession(any())).thenReturn(new SessionService.SessionValue("sessionId", challengeeId, "username", "us"));
        when(mockRemoteDict.getSession("sessionId")).thenReturn(new PlayerEntity(challengeeId, "playerName"));
        when(mockGameService.create(any())).thenReturn("test-id");
        when(mockChallengeDao.delete(anyLong(), anyLong())).thenReturn(1);

        State state = new State();
        state.setChallengeDao(mockChallengeDao);
        state.setGameService(mockGameService);
        state.setRemoteDict(mockRemoteDict);
        state.setSessionService(mockSessionService);

        MockRouter mockRouter = new MockRouter(new FormController(state));
        MockContext mockContext = mockChallengeCtx(challengerId, challengeeId, "ACCEPT");

        // when
        mockRouter.post("/forms/challenge", mockContext);

        // then
        verify(mockChallengeDao, times(1)).delete(challengerId, challengeeId);
        verify(mockGameService, times(1)).create(null);
    }

    @Test
    public void testNoAcceptChallenge() {
        // given
        long challengeeId = 1L;
        long challengerId = 2L;

        ChallengeDao mockChallengeDao = mock(ChallengeDao.class);
        GameService mockGameService = mock(GameService.class);
        RemoteDict mockRemoteDict = mock(RemoteDict.class);
        SessionService mockSessionService = mock(SessionService.class);

        when(mockSessionService.parseSession(any())).thenReturn(new SessionService.SessionValue("sessionId", challengeeId, "username", "us"));
        when(mockRemoteDict.getSession("sessionId")).thenReturn(new PlayerEntity(challengeeId, "playerName"));
        when(mockChallengeDao.delete(anyLong(), anyLong())).thenReturn(0);

        State state = new State();
        state.setChallengeDao(mockChallengeDao);
        state.setGameService(mockGameService);
        state.setRemoteDict(mockRemoteDict);
        state.setSessionService(mockSessionService);

        MockRouter mockRouter = new MockRouter(new FormController(state));
        MockContext mockContext = mockChallengeCtx(challengerId, challengeeId, "ACCEPT");

        // when
        Assertions.assertThrows(StatusCodeException.class, () -> mockRouter.post("/forms/update-challenge", mockContext));

        // then
        verify(mockChallengeDao, times(1)).delete(challengerId, challengeeId);
        verify(mockGameService, times(0)).create(any());
    }

    @Test
    public void testDeleteChallenge() {
        // given
        long challengeeId = 1L;
        long challengerId = 2L;

        ChallengeDao mockChallengeDao = mock(ChallengeDao.class);
        GameService mockGameService = mock(GameService.class);
        RemoteDict mockRemoteDict = mock(RemoteDict.class);
        SessionService mockSessionService = mock(SessionService.class);

        when(mockSessionService.parseSession(any())).thenReturn(new SessionService.SessionValue("sessionId", challengerId, "username", "us"));
        when(mockRemoteDict.getSession("sessionId")).thenReturn(new PlayerEntity(challengerId, "playerName"));
        when(mockChallengeDao.delete(anyLong(), anyLong())).thenReturn(1);

        State state = new State();
        state.setChallengeDao(mockChallengeDao);
        state.setGameService(mockGameService);
        state.setRemoteDict(mockRemoteDict);
        state.setSessionService(mockSessionService);

        MockRouter mockRouter = new MockRouter(new FormController(state));
        MockContext mockContext = mockChallengeCtx(challengerId, challengeeId, "DELETE");

        // when
        mockRouter.post("/forms/challenge", mockContext);

        // then
        verify(mockChallengeDao, times(1)).delete(challengerId, challengeeId);
        verify(mockGameService, times(0)).create(null);
    }

    @Test
    public void testRejectChallenge() {
        // given
        long challengeeId = 1L;
        long challengerId = 2L;

        ChallengeDao mockChallengeDao = mock(ChallengeDao.class);
        GameService mockGameService = mock(GameService.class);
        RemoteDict mockRemoteDict = mock(RemoteDict.class);
        SessionService mockSessionService = mock(SessionService.class);

        when(mockSessionService.parseSession(any())).thenReturn(new SessionService.SessionValue("sessionId", challengeeId, "username", "us"));
        when(mockRemoteDict.getSession("sessionId")).thenReturn(new PlayerEntity(challengeeId, "playerName"));
        when(mockChallengeDao.delete(anyLong(), anyLong())).thenReturn(1);

        State state = new State();
        state.setChallengeDao(mockChallengeDao);
        state.setGameService(mockGameService);
        state.setRemoteDict(mockRemoteDict);
        state.setSessionService(mockSessionService);

        MockRouter mockRouter = new MockRouter(new FormController(state));
        MockContext mockContext = mockChallengeCtx(challengerId, challengeeId, "REJECT");

        // when
        mockRouter.post("/forms/challenge", mockContext);

        // then
        verify(mockChallengeDao, times(1)).delete(challengerId, challengeeId);
        verify(mockGameService, times(0)).create(null);
    }
}
