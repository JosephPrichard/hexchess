package web;

import io.jooby.Context;
import io.jooby.Cookie;
import io.jooby.SameSite;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.security.SecureRandom;

@AllArgsConstructor
public class SessionService {

    private static final SecureRandom RANDOM = new SecureRandom();
    public static final String COOKIE_NAME = "session";
    private static final String CHARACTERS = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";

    public String createId() {
        int length = 100;
        StringBuilder sb = new StringBuilder(length);
        for (int i = 0; i < length; i++) {
            int index = RANDOM.nextInt(CHARACTERS.length());
            sb.append(CHARACTERS.charAt(index));
        }
        return sb.toString();
    }

    @Data
    @NoArgsConstructor
    @AllArgsConstructor
    public static class SessionValue {
        String sessionId;
        String playerId;
        String username;
        String country;
    }

    public Cookie createCookie(String sessionId, String playerId, String username, String country) {
        String sessionCsv = String.format("%s,%s,%s,%s", sessionId, playerId, username, country);
        int maxAgeSecs = 6 * 60 * 60; // 6 hours

        // our cookie is not set to http only because javascript must read it starting a websocket
        return new Cookie(COOKIE_NAME, sessionCsv)
            .setDomain("localhost")
            .setSecure(true)
            .setSameSite(SameSite.STRICT)
            .setMaxAge(maxAgeSecs);
    }

    public Cookie createEmptyCookie() {
        return new Cookie(COOKIE_NAME, "")
            .setDomain("localhost")
            .setSecure(true)
            .setSameSite(SameSite.STRICT)
            .setMaxAge(1);
    }

    public SessionValue parseSession(Context ctx) {
        String cookieStr = ctx.header("Cookie").valueOrNull();

        if (cookieStr == null) {
            return null;
        }
        int delimIndex = cookieStr.indexOf("=");
        if (delimIndex < 0) {
            throw new IllegalArgumentException("Invalid cookie, must contain a key-value pair: " + cookieStr);
        }

        String value = cookieStr.substring(delimIndex + 1);
        if (!value.isEmpty()) {
            value = value.substring(1, value.length() - 1); // strip quotes from
        }

        String[] fields = value.split(",");
        if (fields.length != 4) {
            throw new IllegalArgumentException("Invalid cookie, 'session' must contain 4 fields: " + cookieStr);
        }

        return new SessionValue(fields[0], fields[1], fields[2], fields[3]);
    }
}
