package utils;

import com.zaxxer.hikari.HikariConfig;
import com.zaxxer.hikari.HikariDataSource;
import org.apache.commons.dbutils.QueryRunner;

import javax.sql.DataSource;
import java.io.File;
import java.io.IOException;
import java.net.URL;
import java.nio.file.Files;
import java.sql.SQLException;
import java.util.HashMap;
import java.util.Map;

import static utils.Globals.LOGGER;

public class Config {

    public static HikariDataSource createDataSource() {
        String dbUrl = System.getenv("DB_URL");
        String dbUser = System.getenv("DB_USER");
        String dbPassword = System.getenv("DB_PASSWORD");

        HikariConfig config = new HikariConfig();
        config.setJdbcUrl(dbUrl);
        config.setUsername(dbUser);
        config.setPassword(dbPassword);
        config.setMaximumPoolSize(10);
        config.setAutoCommit(false);
        config.setDriverClassName("org.postgresql.Driver");
        config.setAutoCommit(true);

        return new HikariDataSource(config);
    }

    public static Map<String, byte[]> createFilesMap() {
        Map<String, byte[]> filesMap = new HashMap<>();

        try {
            URL resource = ClassLoader.getSystemResource("database");
            if (resource == null) {
                return filesMap;
            }

            File dir = new File(resource.getFile());
            File[] files = dir.listFiles();
            assert files != null;
            for (File file : files) {
                byte[] data = Files.readAllBytes(file.toPath());
                filesMap.put(file.getName(), data);
            }
        } catch (IOException ex) {
            LOGGER.error("Error occurred while creating files map {}", String.valueOf(ex));
            throw new RuntimeException(ex);
        }

        return filesMap;
    }

    public static void createSchema(DataSource ds) {
        try {
            QueryRunner runner = new QueryRunner(ds);
            runner.execute("DROP SCHEMA public CASCADE; CREATE SCHEMA public;");

            URL resource = ClassLoader.getSystemResource("database");
            File dir = new File(resource.getFile());
            File[] files = dir.listFiles();
            assert files != null;
            for (File file : files) {
                byte[] data = Files.readAllBytes(file.toPath());
                String sql = new String(data);
                runner.update(sql);
            }
        } catch (SQLException | IOException ex) {
            LOGGER.error("Error occurred while creating schema {}", String.valueOf(ex));
            throw new RuntimeException(ex);
        }
    }
}
