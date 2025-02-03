package web;

import io.jooby.Cookie;
import io.jooby.Formdata;
import io.jooby.test.MockContext;
import io.jooby.test.MockResponse;
import io.jooby.test.MockRouter;
import models.Player;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.Test;
import services.GameService;
import services.RemoteDict;
import services.UserDao;

import java.util.concurrent.atomic.AtomicReference;

import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

public class FormRouterTest {
    @Test
    public void testPostSignup() throws UserDao.TakenUsernameException {
        // given
        var accountInst = new UserDao.UserInst("1", "testUser", "testPassword", "USA", 1000, 3, 3);
        var player = new Player("1", "testUser");
        var country = "us";
        var cookie = new Cookie("sessionToken");

        var mockUserDao = mock(UserDao.class);
        var mockSessionService = mock(SessionService.class);

        when(mockUserDao.insert(any(), any())).thenReturn(accountInst);
        when(mockSessionService.createId()).thenReturn("sessionToken");
        when(mockSessionService.createCookie(any(), any(), any(), any())).thenReturn(cookie);

        var state = new State();
        state.setUserDao(mockUserDao);
        state.setSessionService(mockSessionService);

        var mockDict = mock(RemoteDict.class);
        state.setRemoteDict(mockDict);

        var mockRouter = new MockRouter(new FormRouter(state));

        var mockContext = new MockContext();
        var mockForm = Formdata.create(mockContext);
        mockForm.put("username", "testUser");
        mockForm.put("password", "testPassword");
        mockForm.put("duplicate-password", "testPassword");
        mockContext.setForm(mockForm);

        // when
        AtomicReference<MockResponse> response = new AtomicReference<>();
        mockRouter.post("/forms/signup", mockContext, response::set);
        var actualCookie = response.get().getHeaders().get("Set-Cookie");

        // then
        verify(mockUserDao, times(1)).insert("testUser", "testPassword");
        verify(mockDict, times(1)).setSession(anyString(), eq(player), anyLong());
        verify(mockSessionService, times(1)).createCookie(eq("sessionToken"), eq(player.getId()), eq(player.getName()), any());

        Assertions.assertEquals(actualCookie, cookie.toString());
    }

    @Test
    public void testPostLogin() {
        // given
        var player = new Player("1", "testUser");
        var country = "us";
        var cookie = new Cookie("sessionToken");

        var mockUserDao = mock(UserDao.class);
        var mockSessionService = mock(SessionService.class);

        when(mockUserDao.verify(any(), any())).thenReturn(new UserDao.VerifiedPlayer("1", "testUser", country));
        when(mockSessionService.createId()).thenReturn("sessionToken");
        when(mockSessionService.createCookie(any(), any(), any(), any())).thenReturn(cookie);

        var state = new State();
        state.setUserDao(mockUserDao);
        state.setSessionService(mockSessionService);

        var mockDict = mock(RemoteDict.class);
        state.setRemoteDict(mockDict);

        var mockRouter = new MockRouter(new FormRouter(state));

        var mockContext = new MockContext();
        var mockForm = Formdata.create(mockContext);
        mockForm.put("username", "testUser");
        mockForm.put("password", "testPass");
        mockContext.setForm(mockForm);

        // when
        AtomicReference<MockResponse> response = new AtomicReference<>();
        mockRouter.post("/forms/login", mockContext, response::set);
        var actualCookie = response.get().getHeaders().get("Set-Cookie");

        // then
        verify(mockUserDao, times(1)).verify("testUser", "testPass");
        verify(mockDict, times(1)).setSession(anyString(), eq(player), anyLong());
        verify(mockSessionService, times(1)).createCookie(eq("sessionToken"), eq(player.getId()), eq(player.getName()), any());

        Assertions.assertEquals(actualCookie, cookie.toString());
    }

    @Test
    public void testPostCreateGame() {
        // given
        var state = new State();

        var mockgameService = mock(GameService.class);
        when(mockgameService.create(null)).thenReturn("test-id");
        state.setGameService(mockgameService);

        var mockRouter = new MockRouter(new FormRouter(state));

        // when
        var result = mockRouter.post("/forms/create-game");

        // then
        verify(mockgameService, times(1)).create(null);
        Assertions.assertEquals("test-id", result.value());
    }
}
