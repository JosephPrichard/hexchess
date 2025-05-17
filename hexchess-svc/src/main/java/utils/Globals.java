package utils;

import com.fasterxml.jackson.annotation.JsonAutoDetect;
import com.fasterxml.jackson.annotation.PropertyAccessor;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.jsoup.safety.Safelist;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.security.SecureRandom;
import java.util.Random;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

public class Globals {

    public static final Logger LOGGER = LoggerFactory.getLogger("HexChess");

    public static Safelist HTML_SAFELIST = Safelist.basic();

    public static final ExecutorService EXECUTOR = Executors.newThreadPerTaskExecutor(Thread.ofVirtual().name("thread-", 0L).factory());

    public static final ObjectMapper JSON_MAPPER = new ObjectMapper();

    public static final Random RANDOM = new Random();

    public static final String CHARACTERS = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz";
    public static final SecureRandom SECURE_RANDOM = new SecureRandom();
}
