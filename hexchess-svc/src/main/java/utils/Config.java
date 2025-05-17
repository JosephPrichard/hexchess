package utils;

import com.zaxxer.hikari.HikariConfig;
import com.zaxxer.hikari.HikariDataSource;
import org.apache.commons.dbutils.QueryRunner;
import redis.clients.jedis.ConnectionPoolConfig;

import javax.sql.DataSource;
import java.io.File;
import java.io.IOException;
import java.net.URL;
import java.nio.file.Files;
import java.sql.SQLException;
import java.time.Duration;
import java.util.*;

import static utils.Globals.LOGGER;

public class Config {

    public static ConnectionPoolConfig getJedisPoolConfig() {
        ConnectionPoolConfig poolConfig = new ConnectionPoolConfig();
        poolConfig.setMaxTotal(24);
        poolConfig.setMaxIdle(8);
        poolConfig.setMinIdle(0);
        poolConfig.setBlockWhenExhausted(false);
        poolConfig.setMaxWait(Duration.ofSeconds(1));
        poolConfig.setTestWhileIdle(true);
        poolConfig.setTimeBetweenEvictionRuns(Duration.ofSeconds(1));
        return poolConfig;
    }

    public static Map<String, String> readEnvironment() {
        Map<String, String> env = new HashMap<>();

        try {
            URL resource = ClassLoader.getSystemResource(".env");
            File file = new File(resource.getFile());

            // put all .env file members into the env map
            Scanner scanner = new Scanner(file);
            while (scanner.hasNextLine()) {
                String line = scanner.nextLine();
                String[] tokens = line.split("=");
                String key = tokens[0];
                String value = tokens[1];
                env.put(key, value);
            }

            // put all system env variables into the env map
            for (String envName : System.getenv().keySet()) {
                env.put(envName, System.getenv(envName));
            }
        } catch (IOException e) {
            LOGGER.error("Error occurred while reading the .env file", e);
            throw new RuntimeException(e);
        }

        return env;
    }

    public static HikariDataSource createDataSource(Map<String, String> env) {
        String dbUrl = env.get("DB_URL");
        String dbUser = env.get("DB_USER");
        String dbPassword = env.get("DB_PASSWORD");

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

    public static List<String> createCountryList() {
        List<String> countryList = new ArrayList<>();

        URL resource = ClassLoader.getSystemResource("flags");
        if (resource == null) {
            return countryList;
        }

        countryList.add("us");

        File dir = new File(resource.getFile());
        File[] files = dir.listFiles();

        assert files != null;
        for (File file : files) {
            String flagName = file.getName();
            if (flagName.equals("us")) {
                continue;
            }

            int index = flagName.lastIndexOf(".");
            if (index != -1) {
                flagName = flagName.substring(0, index);
            }
            countryList.add(flagName);
        }

        return countryList;
    }

    public static void createSchema(DataSource ds) {
        try {
            QueryRunner runner = new QueryRunner(ds);
            runner.execute("DROP SCHEMA public CASCADE; CREATE SCHEMA public;");

            URL resource = ClassLoader.getSystemResource("sql");
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
