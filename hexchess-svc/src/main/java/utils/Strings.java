package utils;

import org.jsoup.Jsoup;

import java.sql.Timestamp;
import java.time.format.DateTimeFormatter;

import static utils.Globals.HTML_SAFELIST;

public class Strings {
    private static final DateTimeFormatter DATE_FORMATTER = DateTimeFormatter.ofPattern("MM/dd/yyyy");

    public static String sanitize(String input) {
        return input != null ? Jsoup.clean(input, HTML_SAFELIST) : null;
    }

    public static String formatTimestamp(Timestamp timestamp) {
        if (timestamp == null) {
            return "";
        }
        return timestamp.toLocalDateTime().format(DATE_FORMATTER);
    }
}
