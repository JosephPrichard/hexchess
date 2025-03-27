package dao;

import at.favre.lib.crypto.bcrypt.BCrypt;
import lombok.AllArgsConstructor;
import lombok.Data;
import models.RankedUser;
import models.User;
import org.apache.commons.dbutils.DbUtils;
import org.apache.commons.dbutils.QueryRunner;
import org.apache.commons.dbutils.ResultSetHandler;
import org.apache.commons.dbutils.handlers.BeanHandler;
import org.apache.commons.dbutils.handlers.BeanListHandler;
import org.apache.commons.dbutils.handlers.ScalarHandler;

import javax.sql.DataSource;
import java.security.NoSuchAlgorithmException;
import java.security.SecureRandom;
import java.sql.*;
import java.util.Base64;
import java.util.List;
import java.util.UUID;
import java.util.stream.Collectors;

import static utils.Globals.LOGGER;

public class UserDao {

    private static final ResultSetHandler<User> USER_MAPPER = new BeanHandler<>(User.class);
    private static final ResultSetHandler<List<User>> USER_LIST_MAPPER = new BeanListHandler<>(User.class);
    private static final ResultSetHandler<Integer> INT_MAPPER = new ScalarHandler<>();

    private final QueryRunner runner;

    public UserDao(DataSource ds) {
        runner = new QueryRunner(ds);
    }

    @Data
    @AllArgsConstructor
    public static class UserInst {
        public String newId;
        public String username;
        public String password;
        public String country;
        public float elo;
        public int wins;
        public int losses;
    }

    public static class TakenUsernameException extends RuntimeException {
    }

    public UserInst insert(String username, String password) throws TakenUsernameException {
        UserInst inst = new UserInst(UUID.randomUUID().toString(), username, password, "USA", User.START_ELO, 0, 0);
        insert(inst);
        return inst;
    }

    public static String generateSalt() {
        try {
            byte[] salt = new byte[16];
            SecureRandom.getInstanceStrong().nextBytes(salt);
            return Base64.getEncoder().encodeToString(salt);
        } catch (NoSuchAlgorithmException ex) {
            throw new RuntimeException(ex);
        }
    }

    public void insert(UserInst inst) throws TakenUsernameException {
        String salt = generateSalt();
        String saltedPassword = inst.getPassword() + salt;
        String hashedPassword = BCrypt.withDefaults().hashToString(12, saltedPassword.toCharArray());

        String sql = """
            BEGIN;
                INSERT INTO users (id, username, country, elo, highestElo, wins, losses, password, salt)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);
                UPDATE users_metadata SET count = count + 1 WHERE id = 1;
            END;
            """;
        try {
            runner.execute(sql,
                inst.getNewId(),
                inst.getUsername(),
                inst.getCountry(),
                inst.getElo(),
                inst.getElo(),
                inst.getWins(),
                inst.getLosses(),
                hashedPassword,
                salt);
            LOGGER.info("Inserted user={}", inst);
        } catch (SQLException ex) {
            SQLException nextEx = ex.getNextException();
            if ("23505".equals(nextEx.getSQLState())) {
                LOGGER.warn("Username is already taken={}", inst);
                throw new TakenUsernameException();
            }

            LOGGER.error("Failed to insert user={}", inst, ex);
            throw new RuntimeException(ex);
        }
    }

    @Data
    @AllArgsConstructor
    public static class VerifiedPlayer {
        public String id;
        public String username;
        public String country;
    }

    public VerifiedPlayer verify(String username, String inputPassword) {
        String sql = "SELECT id, username, country, password, salt FROM users WHERE UPPER(username) = UPPER(?)";

        Connection conn = null;
        PreparedStatement stmt = null;
        ResultSet rs = null;
        try {
            conn = runner.getDataSource().getConnection();
            stmt = conn.prepareStatement(sql);
            stmt.setString(1, username);

            rs = stmt.executeQuery();

            if (!rs.next()) {
                LOGGER.warn("No user found with username={}", username);
                return null;
            }
            String salt = rs.getString("salt");
            String password = rs.getString("password");
            String id = rs.getString("id");
            String usernameOut = rs.getString("username");
            String country = rs.getString("country");

            String saltedPassword = inputPassword + salt;
            BCrypt.Result result = BCrypt.verifyer().verify(saltedPassword.toCharArray(), password);

            LOGGER.info("Password verification {} for username={}", result.verified ? "successful" : "failed", usernameOut);
            return result.verified ? new VerifiedPlayer(id, usernameOut, country) : null;
        } catch (SQLException ex) {
            LOGGER.error("Failed to select user credentials for user={}", username, ex);
            DbUtils.rollbackAndCloseQuietly(conn);
            throw new RuntimeException(ex);
        } finally {
            DbUtils.closeQuietly(conn);
            DbUtils.closeQuietly(stmt);
            DbUtils.closeQuietly(rs);
        }
    }

    public void updateUser(String id, String newUsername, String newCountry, String newBio) {
        if (newUsername == null && newCountry == null && newBio == null) {
            LOGGER.info("No fields provided for user update.");
            return;
        }

        String sql = """
            UPDATE users
            SET username = COALESCE(?, username), country = COALESCE(?, country), bio = COALESCE(?, bio)
            WHERE id = ?;
            """;

        try {
            runner.execute(sql, newUsername, newCountry, newBio, id);
            LOGGER.info("Updated user data with id={}", id);
        } catch (SQLException ex) {
            LOGGER.error("Failed to update user with id={}", id, ex);
            throw new RuntimeException(ex);
        }
    }

    public void updatePassword(String id, String newPassword) {
        String salt = generateSalt();
        String saltedPassword = newPassword + salt;
        String hashedPassword = BCrypt.withDefaults().hashToString(12, saltedPassword.toCharArray());

        String sql = "UPDATE users SET password = ?, salt = ? WHERE id = ?";
        try {
            runner.execute(sql, hashedPassword, salt, id);
            LOGGER.info("Updated user password with id={}", id);
        } catch (SQLException ex) {
            LOGGER.error("Failed to update user with id={}", id, ex);
            throw new RuntimeException(ex);
        }
    }

    @Data
    @AllArgsConstructor
    public static class EloChangeSet {
        public double winEloDiff;
        public double loseEloDiff;

        public void roundElo() {
            winEloDiff = Math.round(winEloDiff);
            loseEloDiff = Math.round(loseEloDiff);
        }
    }

    public EloChangeSet updateStats(String winId, String loseId) {
        String sql = "CALL updateStats(?, ?, ?, ?)";

        Connection conn = null;
        CallableStatement stmt = null;
        try {
            conn = runner.getDataSource().getConnection();

            stmt = conn.prepareCall(sql);
            stmt.setString(1, winId);
            stmt.setString(2, loseId);
            stmt.registerOutParameter(3, Types.NUMERIC);
            stmt.registerOutParameter(4, Types.NUMERIC);

            stmt.execute();

            double winEloDiff = stmt.getBigDecimal(3).doubleValue();
            double loseEloDiff = stmt.getBigDecimal(4).doubleValue();

            LOGGER.info("Updated stats: winId={} loseId={}, winEloDiff={}, loseEloDiff={}", winId, loseId, winEloDiff, loseEloDiff);
            return new EloChangeSet(winEloDiff, loseEloDiff);
        } catch (SQLException ex) {
            LOGGER.error("Failed to update stats for winId={}, loseId={}", winId, loseId, ex);
            DbUtils.rollbackAndCloseQuietly(conn);
            throw new RuntimeException(ex);
        } finally {
            DbUtils.closeQuietly(conn);
            DbUtils.closeQuietly(stmt);
        }
    }

    public User getById(String id) {
        String sql = """
            SELECT id, username, country, elo, highestElo, wins, losses, bio, joinedOn
            FROM users
            WHERE id = ?
            """;
        try {
            User user = runner.query(sql, USER_MAPPER, id);
            LOGGER.info("Selected user={} by id={}", user, id);
            return user;
        } catch (SQLException ex) {
            LOGGER.error("Failed to fetch user by id={}", id, ex);
            throw new RuntimeException(ex);
        }
    }

    public User getByIdWithRank(String id) {
        String sql = """
            SELECT u1.id, u1.username, u1.country, u1.elo, u1.highestElo, u1.wins, u1.losses, u1.joinedOn, u1.bio,
                (SELECT COUNT(*) FROM users u2 WHERE u2.elo >= u1.elo) as rank
            FROM users u1
            WHERE u1.id = ?
            """;
        try {
            User user = runner.query(sql, USER_MAPPER, id);
            LOGGER.info("Selected user={} with rank by id={}", user, id);
            return user;
        } catch (SQLException ex) {
            LOGGER.error("Failed to fetch user with rank by id={}", id, ex);
            throw new RuntimeException(ex);
        }
    }

    public List<User> getByRanks(List<RankedUser> users) {
        return getByIds(users.stream().map(RankedUser::getId).toList());
    }

    public List<User> getByIds(List<String> ids) {
        String sql = """
            SELECT id, username, country, elo, wins, losses
            FROM users
            WHERE id = ANY (?)
            """;

        String idsStr = ids.stream().collect(Collectors.joining(",", "[", "]"));

        Connection conn = null;
        PreparedStatement stmt = null;
        ResultSet rs = null;
        try {
            conn = runner.getDataSource().getConnection();
            stmt = conn.prepareStatement(sql);

            stmt.setArray(1, conn.createArrayOf("VARCHAR", ids.toArray()));
            rs = stmt.executeQuery();

            List<User> users = USER_LIST_MAPPER.handle(rs);
            LOGGER.info("Selected users={} by ids={}", users, idsStr);
            return users;
        } catch (SQLException e) {
            LOGGER.error("Failed to select users by ids={}", idsStr);
            throw new RuntimeException(e);
        } finally {
            DbUtils.closeQuietly(conn);
            DbUtils.closeQuietly(stmt);
            DbUtils.closeQuietly(rs);
        }
    }

    public List<User> getAll() {
        String sql = "SELECT id, username, country, elo, wins, losses FROM users";
        try {
            List<User> users = runner.query(sql, USER_LIST_MAPPER);
            LOGGER.info("Selected ALL records={} from the user table", users);
            return users;
        } catch (SQLException ex) {
            LOGGER.error("Failed to select ALL records from the user table");
            throw new RuntimeException(ex);
        }
    }

    public List<User> getLeaderboard(int page, int perPage) {
        String sql = """
            SELECT id, username, country, elo, wins, losses
            FROM users
            ORDER BY elo DESC LIMIT ? OFFSET ?
            """;

        page = Math.max(page, 1);
        int offset = (page - 1) * perPage;

        try {
            List<User> users = runner.query(sql, USER_LIST_MAPPER, perPage, offset);
            for (int i = 0; i < users.size(); i++) {
                users.get(i).setRank((page - 1) * perPage + i + 1);
            }
            LOGGER.info("Selected leaderboard={} for page={}, perPage={}", users, page, perPage);
            return users;
        } catch (SQLException ex) {
            LOGGER.error("Failed to select leaderboard for page={}, perPage={}", page, perPage);
            throw new RuntimeException(ex);
        }
    }

    public List<User> searchByName(String name, int page, int perPage) {
        String sql = """
            SELECT id, username, country, elo, wins, losses, (username <-> ?) as rank
            FROM users
            WHERE 1 = 1 AND username % ?
            ORDER BY rank DESC LIMIT ? OFFSET ?
            """;

        page = Math.max(page, 1);
        int offset = (page - 1) * perPage;

        try {
            List<User> users = runner.query(sql, USER_LIST_MAPPER, name, name, perPage, offset);
            for (int i = 0; i < users.size(); i++) {
                users.get(i).setRank(i + 1);
            }
            LOGGER.info("Selected users={} by name for name={}, page={}, perPage={}", users, name, page, perPage);
            return users;
        } catch (SQLException ex) {
            LOGGER.error("Failed to select users by name for name={}, page={}, perPage={}", name, page, perPage);
            throw new RuntimeException(ex);
        }
    }

    public int countUsers() {
        String sql = "SELECT count FROM users_metadata";
        try {
            Integer count = runner.query(sql, INT_MAPPER);
            LOGGER.info("Counted user table records with count={}", count);
            return count;
        } catch (SQLException ex) {
            LOGGER.error("Failed to count user table records");
            throw new RuntimeException(ex);
        }
    }

    public int countPages(int perPage) {
        return countUsers() / perPage + 1;
    }
}