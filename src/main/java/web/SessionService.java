package web;

import io.jooby.Cookie;
import io.jooby.SameSite;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.security.SecureRandom;
import java.util.function.Function;

@AllArgsConstructor
public class SessionService {

    private static final SecureRandom RANDOM = new SecureRandom();
    public static final String COOKIE_NAME = "session";
    private static final String CHARACTERS = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";

    public String createId() {
        var length = 100;
        var sb = new StringBuilder(length);
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
        var sessionCsv = String.format("%s,%s,%s,%s", sessionId, playerId, username, country);
        var maxAgeSecs = 6 * 60 * 60; // 6 hours

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

    private static String getFieldOrNull(String[] fields, int index, Function<String, String> map) {
        return fields.length > index ? map.apply(fields[index]) : null;
    }

    private static String getFieldOrNull(String[] fields, int index) {
        return getFieldOrNull(fields, index, Function.identity());
    }

    public SessionValue getSession(String cookieStr) {
        if (cookieStr == null) {
            return null;
        }
        var delimIndex = cookieStr.indexOf("=");
        if (delimIndex < 0) {
            return null;
        }
        var fields = cookieStr.substring(delimIndex + 1).split(",");
        return new SessionValue(
            getFieldOrNull(fields, 0, s -> s.substring(1)),
            getFieldOrNull(fields, 1),
            getFieldOrNull(fields, 2, s -> s.substring(0, fields[2].length() - 1)),
            getFieldOrNull(fields, 3));
    }
}
