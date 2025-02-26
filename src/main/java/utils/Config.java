package utils;

import com.zaxxer.hikari.HikariConfig;
import com.zaxxer.hikari.HikariDataSource;
import org.apache.commons.dbutils.QueryRunner;
import web.Router;

import javax.sql.DataSource;
import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStreamReader;
import java.net.URISyntaxException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.sql.SQLException;
import java.util.HashMap;
import java.util.Map;
import java.util.Objects;
import java.util.stream.Stream;

import static utils.Globals.LOGGER;

public class Config {

    public static HikariDataSource createDataSource() {
        var dbUrl = System.getenv("DB_URL");
        var dbUser = System.getenv("DB_USER");
        var dbPassword = System.getenv("DB_PASSWORD");

        var config = new HikariConfig();
        config.setJdbcUrl(dbUrl);
        config.setUsername(dbUser);
        config.setPassword(dbPassword);
        config.setMaximumPoolSize(10);
        config.setAutoCommit(false);
        config.setDriverClassName("org.postgresql.Driver");

        return new HikariDataSource(config);
    }

    public static Map<String, byte[]> createFilesMap() {
        Map<String, byte[]> files = new HashMap<>();

        var classLoader = Thread.currentThread().getContextClassLoader();
        try {
            var resource = classLoader.getResource("flags");
            if (resource == null) {
                return files;
            }

            var resourcePath = Paths.get(resource.toURI());
            try (Stream<Path> paths = Files.walk(resourcePath)) {
                paths.filter(Files::isRegularFile).forEach(filePath -> {
                    try (var inputStream = classLoader.getResourceAsStream("flags/" + filePath.getFileName().toString())) {
                        if (inputStream != null) {
                            files.put(filePath.getFileName().toString(), inputStream.readAllBytes());
                        }
                    } catch (IOException ex) {
                        LOGGER.error("Error occurred while stepping through files {}", String.valueOf(ex));
                        throw new RuntimeException(ex);
                    }
                });
            }
        } catch (URISyntaxException | IOException ex) {
            LOGGER.error("Error occurred while creating files map {}", String.valueOf(ex));
            throw new RuntimeException(ex);
        }

        return files;
    }

    public static void executeQuery(QueryRunner runner, String path) {
        var classLoader = Thread.currentThread().getContextClassLoader();
        try (var inputStream = classLoader.getResourceAsStream(path)) {
            if (inputStream == null) {
                throw new RuntimeException("Expected input stream to be non null");
            }
            var sql = new String(inputStream.readAllBytes());
            runner.update(sql);
        } catch (SQLException | IOException ex) {
            LOGGER.error("Error occurred while executing query {}", String.valueOf(ex));
            throw new RuntimeException(ex);
        }
    }

    public static void createSchema(DataSource ds) {
        try {
            var runner = new QueryRunner(ds);
            runner.execute("DROP SCHEMA public CASCADE; CREATE SCHEMA public;");
            executeQuery(runner, "database/schema.sql");
            executeQuery(runner, "database/updateStats.sql");
        } catch (SQLException ex) {
            LOGGER.error("Error occurred while creating schema {}", String.valueOf(ex));
            throw new RuntimeException(ex);
        }
    }
}
