package utils;

import com.fasterxml.jackson.annotation.JsonAutoDetect;
import com.fasterxml.jackson.annotation.PropertyAccessor;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.jsoup.safety.Safelist;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.List;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

public class Globals {
    public static Safelist HTML_SAFELIST = Safelist.basic();
    public static final Logger LOGGER = LoggerFactory.getLogger("HexChess");
    public static final String GREEN_COLOR = "green-color";
    public static final String RED_COLOR = "red-color";
    public static final String YELLOW_COLOR = "yellow-color";
    public static final ExecutorService EXECUTOR = Executors.newThreadPerTaskExecutor(Thread.ofVirtual().name("thread-", 0L).factory());
    public static final ObjectMapper JSON_MAPPER = new ObjectMapper()
        .setVisibility(PropertyAccessor.ALL, JsonAutoDetect.Visibility.NONE)
        .setVisibility(PropertyAccessor.FIELD, JsonAutoDetect.Visibility.ANY);
}
