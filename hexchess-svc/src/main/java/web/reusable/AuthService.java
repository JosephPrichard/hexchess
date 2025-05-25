package web.reusable;

import io.jooby.Context;
import io.jooby.Cookie;
import io.jooby.SameSite;
import io.jooby.StatusCode;
import io.jooby.exception.StatusCodeException;
import lombok.AllArgsConstructor;
import models.state.Player;
import services.daos.DictionaryDao;

import java.security.SecureRandom;

import static web.WebConstants.ERROR_REQUIRED_LOGIN;
import static web.WebConstants.ERROR_SESSION_EXPIRED;

@AllArgsConstructor
public class AuthService {

    private static final SecureRandom RANDOM = new SecureRandom();
    public static final String SESSION_COOKIE_NAME = "session";
    public static final int MAX_AGE_COOKIE = 6 * 60 * 60;
    private static final String CHARACTERS = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";

    private final DictionaryDao dictionaryDao;
    private final String cookieDomain;

    public String createSessionId() {
        int length = 100;
        StringBuilder sb = new StringBuilder(length);
        for (int i = 0; i < length; i++) {
            int index = RANDOM.nextInt(CHARACTERS.length());
            sb.append(CHARACTERS.charAt(index));
        }
        return sb.toString();
    }

    public Cookie createCookie(String value, long maxAge) {
        return new Cookie(SESSION_COOKIE_NAME, value)
            .setDomain(cookieDomain)
            .setSecure(true)
            .setSameSite(SameSite.STRICT)
            .setHttpOnly(true)
            .setMaxAge(maxAge);
    }

    public Cookie createSessionCookie(String sessionId) {
        return createCookie(sessionId, MAX_AGE_COOKIE);
    }

    public Cookie createEmptyCookie() {
        return createCookie("", 1);
    }

    public String parseSession(Context ctx) {
        String cookieStr = ctx.header("Cookie").valueOrNull();
        if (cookieStr == null) {
//            LOGGER.warn("Request does not contain a cookie header");
            return null;
        }

        String[] cookieTokens = cookieStr.split(";");
        if (cookieTokens.length == 0) {
//            LOGGER.warn("Request does not contain any cookies in the cookie header");
            return null;
        }

        String sessionStr = null;
        for (String token : cookieTokens) {
            int delimIndex = token.indexOf("=");
            String nameStr = token.substring(0, delimIndex);
            if (nameStr.equals(SESSION_COOKIE_NAME)) {
                sessionStr = token.substring(delimIndex + 1).replaceAll("\\s+","");
            }
        }

//        LOGGER.info("Parsed cookie with value={} from cookie header", sessionStr);

        return sessionStr;
    }

    public Player getSessionPlayer(Context ctx) {
        String sessionId = parseSession(ctx);
        if (sessionId == null) {
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, ERROR_REQUIRED_LOGIN);
        }
        Player player = dictionaryDao.getSession(sessionId);
        if (player == null) {
            ctx.setResponseCookie(createEmptyCookie());
            throw new StatusCodeException(StatusCode.UNAUTHORIZED, ERROR_SESSION_EXPIRED);
        }
        return player;
    }
}
