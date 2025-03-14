package web;

import dao.ChallengeDao;
import io.jooby.Cookie;
import io.jooby.Formdata;
import io.jooby.test.MockContext;
import io.jooby.test.MockResponse;
import io.jooby.test.MockRouter;
import io.jooby.test.MockValue;
import models.Challenge;
import models.Player;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.Test;
import services.GameService;
import services.RemoteDict;
import dao.UserDao;

import java.util.concurrent.atomic.AtomicReference;

import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

public class FormRouterTest {
    @Test
    public void testPostRegister() throws UserDao.TakenUsernameException {
        // given
        UserDao.UserInst accountInst = new UserDao.UserInst("1", "testUser", "testPassword", "USA", 1000, 3, 3);
        Player player = new Player("1", "testUser");
        Cookie cookie = new Cookie("sessionToken");

        UserDao mockUserDao = mock(UserDao.class);
        SessionService mockSessionService = mock(SessionService.class);

        when(mockUserDao.insert(any(), any())).thenReturn(accountInst);
        when(mockSessionService.createId()).thenReturn("sessionToken");
        when(mockSessionService.createCookie(any(), any(), any(), any())).thenReturn(cookie);

        State state = new State();
        state.setUserDao(mockUserDao);
        state.setSessionService(mockSessionService);

        RemoteDict mockDict = mock(RemoteDict.class);
        state.setRemoteDict(mockDict);

        MockRouter mockRouter = new MockRouter(new FormRouter(state));

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
        Player player = new Player("1", "testUser");
        String country = "us";
        Cookie cookie = new Cookie("sessionToken");

        UserDao mockUserDao = mock(UserDao.class);
        SessionService mockSessionService = mock(SessionService.class);

        when(mockUserDao.verify(any(), any())).thenReturn(new UserDao.VerifiedPlayer("1", "testUser", country));
        when(mockSessionService.createId()).thenReturn("sessionToken");
        when(mockSessionService.createCookie(any(), any(), any(), any())).thenReturn(cookie);

        State state = new State();
        state.setUserDao(mockUserDao);
        state.setSessionService(mockSessionService);

        RemoteDict mockDict = mock(RemoteDict.class);
        state.setRemoteDict(mockDict);

        MockRouter mockRouter = new MockRouter(new FormRouter(state));

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

        MockRouter mockRouter = new MockRouter(new FormRouter(state));

        // when
        MockValue result = mockRouter.post("/forms/create-game");

        // then
        verify(mockGameService, times(1)).create(null);
        Assertions.assertEquals("test-id", result.value());
    }

    @Test
    public void testAcceptChallenge() {
        // given
        String challengeeId = "challengeeId";
        String challengerId = "challengerId";

        ChallengeDao mockChallengeDao = mock(ChallengeDao.class);
        GameService mockGameService = mock(GameService.class);
        RemoteDict mockRemoteDict = mock(RemoteDict.class);
        SessionService mockSessionService = mock(SessionService.class);

        when(mockSessionService.parseSession(any())).thenReturn(new SessionService.SessionValue("sessionId", challengeeId, "username", "us"));
        when(mockRemoteDict.getSession("sessionId")).thenReturn(new Player(challengeeId, "playerName"));
        when(mockGameService.create(null)).thenReturn("test-id");
        when(mockChallengeDao.updateStatus(any(), any(), anyInt())).thenReturn(true);

        State state = new State();
        state.setChallengeDao(mockChallengeDao);
        state.setGameService(mockGameService);
        state.setRemoteDict(mockRemoteDict);
        state.setSessionService(mockSessionService);

        MockRouter mockRouter = new MockRouter(new FormRouter(state));

        MockContext mockContext = new MockContext();
        Formdata mockForm = Formdata.create(mockContext);
        mockForm.put("challengeeId", challengeeId);
        mockForm.put("challengerId", challengerId);
        mockForm.put("action", "ACCEPT");
        mockContext.setForm(mockForm);

        // when
        MockValue result = mockRouter.post("/forms/update-challenge", mockContext);

        // then
        verify(mockChallengeDao, times(1)).updateStatus(challengeeId, challengerId, Challenge.Status.ACCEPTED);
        verify(mockGameService, times(1)).create(null);
        Assertions.assertEquals("test-id", result.value());
    }

    @Test
    public void testDeleteChallenge() {
        // given
        String challengeeId = "challengeeId";
        String challengerId = "challengerId";

        ChallengeDao mockChallengeDao = mock(ChallengeDao.class);
        GameService mockGameService = mock(GameService.class);
        RemoteDict mockRemoteDict = mock(RemoteDict.class);
        SessionService mockSessionService = mock(SessionService.class);

        when(mockSessionService.parseSession(any())).thenReturn(new SessionService.SessionValue("sessionId", challengerId, "username", "us"));
        when(mockRemoteDict.getSession("sessionId")).thenReturn(new Player(challengerId, "playerName"));

        State state = new State();
        state.setChallengeDao(mockChallengeDao);
        state.setGameService(mockGameService);
        state.setRemoteDict(mockRemoteDict);
        state.setSessionService(mockSessionService);

        MockRouter mockRouter = new MockRouter(new FormRouter(state));

        MockContext mockContext = new MockContext();
        Formdata mockForm = Formdata.create(mockContext);
        mockForm.put("challengeeId", challengeeId);
        mockForm.put("challengerId", challengerId);
        mockForm.put("action", "DELETE");
        mockContext.setForm(mockForm);

        // when
        MockValue result = mockRouter.post("/forms/update-challenge", mockContext);

        // then
        verify(mockChallengeDao, times(1)).deleteChallenge(challengeeId, challengerId);
        verify(mockGameService, times(0)).create(null);
        Assertions.assertEquals("Successfully updated challenge!", result.value());
    }

    @Test
    public void testRejectChallenge() {
        // given
        String challengeeId = "challengeeId";
        String challengerId = "challengerId";

        ChallengeDao mockChallengeDao = mock(ChallengeDao.class);
        GameService mockGameService = mock(GameService.class);
        RemoteDict mockRemoteDict = mock(RemoteDict.class);
        SessionService mockSessionService = mock(SessionService.class);

        when(mockSessionService.parseSession(any())).thenReturn(new SessionService.SessionValue("sessionId", challengeeId, "username", "us"));
        when(mockRemoteDict.getSession("sessionId")).thenReturn(new Player(challengeeId, "playerName"));

        State state = new State();
        state.setChallengeDao(mockChallengeDao);
        state.setGameService(mockGameService);
        state.setRemoteDict(mockRemoteDict);
        state.setSessionService(mockSessionService);

        MockRouter mockRouter = new MockRouter(new FormRouter(state));

        MockContext mockContext = new MockContext();
        Formdata mockForm = Formdata.create(mockContext);
        mockForm.put("challengeeId", challengeeId);
        mockForm.put("challengerId", challengerId);
        mockForm.put("action", "REJECT");
        mockContext.setForm(mockForm);

        // when
        MockValue result = mockRouter.post("/forms/update-challenge", mockContext);

        // then
        verify(mockChallengeDao, times(1)).updateStatus(challengeeId, challengerId, Challenge.Status.REJECTED);
        verify(mockGameService, times(0)).create(null);
        Assertions.assertEquals("Successfully updated challenge!", result.value());
    }
}
