package web;

import io.jooby.Context;
import io.jooby.internal.SingleValue;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.Test;

import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;
import static web.AuthService.SESSION_COOKIE_NAME;

public class AuthServiceTest {

    @Test
    public void testParseSession() {
        // given
        Context ctx = mock(Context.class);

        String testCookie = String.format(";%s=sessionValue;;test=testValue;;", SESSION_COOKIE_NAME);

        when(ctx.header("Cookie")).thenReturn(new SingleValue(ctx, "Cookie", testCookie));

        // when
        String session = AuthService.parseSession(ctx);

        // then
        Assertions.assertEquals("sessionValue", session);
    }
}
