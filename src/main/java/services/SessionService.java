package services;

import com.fasterxml.jackson.core.JsonProcessingException;
import io.jooby.Context;
import io.jooby.Cookie;
import io.jooby.SameSite;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.security.SecureRandom;

import static utils.Globals.JSON_MAPPER;
import static utils.Globals.LOGGER;

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
        long userId;
        String username;
        String country;
    }

    public Cookie createCookie(String sessionId, long userId, String username, String country) {
        return createCookie(new SessionValue(sessionId, userId, username, country));
    }

    public Cookie createCookie(SessionValue sessionValue) {
        try {
            String sessionJson = JSON_MAPPER.writeValueAsString(JSON_MAPPER.writeValueAsString(sessionValue));
            int maxAgeSecs = 6 * 60 * 60; // 6 hours

            // our cookie is not set to http only because javascript must read it starting a websocket
            return new Cookie(COOKIE_NAME, sessionJson)
                .setDomain("localhost")
                .setSecure(true)
                .setSameSite(SameSite.STRICT)
                .setMaxAge(maxAgeSecs);
        } catch (JsonProcessingException e) {
            LOGGER.error("Failed to serialize session value={}", sessionValue, e);
            throw new RuntimeException(e);
        }
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

        String[] cookieTokens = cookieStr.split(";");
        if (cookieTokens.length == 0) {
            return null;
        }
        String token = cookieTokens[0]; // we're only expecting one cookie here, so just take the first one

        int delimIndex = token.indexOf("=");
        String sessionStr = token.substring(delimIndex + 1);

        try {
            return JSON_MAPPER.readValue(JSON_MAPPER.readValue(sessionStr, String.class), SessionValue.class);
        } catch (JsonProcessingException e) {
            LOGGER.error("Failed to deserialize session value={}", sessionStr, e);
            throw new RuntimeException(e);
        }
    }
}
