package web;

import io.jooby.Context;
import io.jooby.internal.SingleValue;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.Test;

import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

public class SessionServiceTest {

    @Test
    public void parseSession() {
        String sessionJson = "\"{\\\"sessionId\\\":\\\"session-id\\\",\\\"userId\\\":1,\\\"username\\\":\\\"username-value\\\",\\\"country\\\":\\\"country-name\\\"}\"";

        Context ctx = mock(Context.class);
        when(ctx.header("Cookie")).thenReturn(new SingleValue(ctx, "Cookie", String.format("session=%s;key1=value1;key2=value2", sessionJson)));

        SessionService sessionService = new SessionService();

        SessionService.SessionValue sessionValue = sessionService.parseSession(ctx);

        SessionService.SessionValue expected = new SessionService.SessionValue("session-id", 1, "username-value", "country-name");
        Assertions.assertEquals(expected, sessionValue);
    }
}
