package services.daos;

import at.favre.lib.crypto.bcrypt.BCrypt;
import lombok.*;
import models.entities.RankedEntity;
import models.entities.UserEntity;
import org.apache.commons.dbutils.DbUtils;
import org.apache.commons.dbutils.QueryRunner;
import org.apache.commons.dbutils.ResultSetHandler;
import org.apache.commons.dbutils.handlers.BeanHandler;
import org.apache.commons.dbutils.handlers.BeanListHandler;

import javax.sql.DataSource;
import java.security.NoSuchAlgorithmException;
import java.security.SecureRandom;
import java.sql.*;
import java.util.Arrays;
import java.util.Base64;
import java.util.List;
import java.util.stream.Collectors;

import static utils.Globals.LOG;

public class UserDao {

    private static final ResultSetHandler<UserEntity> USER_MAPPER = new BeanHandler<>(UserEntity.class);
    private static final ResultSetHandler<List<UserEntity>> USER_LIST_MAPPER = new BeanListHandler<>(UserEntity.class);
    private static final ResultSetHandler<VerifiedUser> VERIFIED_USER_MAPPER = new BeanHandler<>(VerifiedUser.class);

    private final QueryRunner runner;

    public UserDao(DataSource ds) {
        runner = new QueryRunner(ds);
    }

    public record UserInst(String username, String password, String country, float elo, int wins, int losses) {}

    public static class TakenUsernameException extends RuntimeException {
    }

    public UserEntity insert(String username, String password) throws TakenUsernameException {
        UserInst inst = new UserInst(username, password, "us", UserEntity.START_ELO, 0, 0);
        return insert(inst);
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

    public UserEntity insert(UserInst inst) throws TakenUsernameException {
        String salt = generateSalt();
        String saltedPassword = inst.password() + salt;
        String hashedPassword = BCrypt.withDefaults().hashToString(12, saltedPassword.toCharArray());

        String sql = """
            INSERT INTO users (username, country, elo, highestElo, wins, losses, password, salt)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?) RETURNING id, username, country, elo, highestElo, wins, losses, bio;
            """;
        try {
            UserEntity user = runner.query(sql, USER_MAPPER,
                inst.username(),
                inst.country(),
                inst.elo(),
                inst.elo(),
                inst.wins(),
                inst.losses(),
                hashedPassword,
                salt);
            LOG.info("Created new user={}", user);
            return user;
        } catch (SQLException ex) {
            SQLException nextEx = ex.getNextException();
            if ("23505".equals(nextEx.getSQLState())) {
                LOG.warn("Username is already taken={}", inst);
                throw new TakenUsernameException();
            }

            LOG.error("Failed to insert user={}", inst, ex);
            throw new RuntimeException(ex);
        }
    }

    @Data
    @NoArgsConstructor
    @AllArgsConstructor
    public static class VerifiedUser {
        private long id;
        private String username;
        private String country;
        private float elo;
    }

    public VerifiedUser verify(String username, String inputPassword) {
        String sql = "SELECT id, username, country, elo, password, salt FROM users WHERE UPPER(username) = UPPER(?)";

        Connection conn = null;
        PreparedStatement stmt = null;
        ResultSet rs = null;
        try {
            conn = runner.getDataSource().getConnection();
            stmt = conn.prepareStatement(sql);
            stmt.setString(1, username);

            rs = stmt.executeQuery();

            if (!rs.next()) {
                LOG.warn("No user found with username={}", username);
                return null;
            }
            String salt = rs.getString("salt");
            String password = rs.getString("password");
            long id = rs.getLong("id");
            String usernameOut = rs.getString("username");
            String country = rs.getString("country");
            float elo = rs.getFloat("elo");

            String saltedPassword = inputPassword + salt;
            BCrypt.Result result = BCrypt.verifyer().verify(saltedPassword.toCharArray(), password);

            LOG.info("Password verification {} for username={}", result.verified ? "successful" : "failed", usernameOut);
            return result.verified ? new VerifiedUser(id, usernameOut, country, elo) : null;
        } catch (SQLException ex) {
            LOG.error("Failed to select user credentials for user={}", username, ex);
            DbUtils.rollbackAndCloseQuietly(conn);
            throw new RuntimeException(ex);
        } finally {
            DbUtils.closeQuietly(conn);
            DbUtils.closeQuietly(stmt);
            DbUtils.closeQuietly(rs);
        }
    }

    public VerifiedUser updateUser(long id, String newUsername, String newCountry, String newBio) {
        if (newUsername == null && newCountry == null && newBio == null) {
            LOG.info("No fields provided for user update.");
            return null;
        }

        String sql = """
            UPDATE users
            SET username = COALESCE(?, username), country = COALESCE(?, country), bio = COALESCE(?, bio)
            WHERE id = ?
            RETURNING *
            """;

        try {
            VerifiedUser user = runner.query(sql, VERIFIED_USER_MAPPER, newUsername, newCountry, newBio, id);
            LOG.info("Updated user data with id={} to newUsername={}, newCountry={}, newBio={}", id, newUsername, newCountry, newBio);
            return user;
        } catch (SQLException ex) {
            LOG.error("Failed to update user with id={}", id, ex);
            throw new RuntimeException(ex);
        }
    }

    public void updatePassword(long id, String newPassword) {
        String salt = generateSalt();
        String saltedPassword = newPassword + salt;
        String hashedPassword = BCrypt.withDefaults().hashToString(12, saltedPassword.toCharArray());

        String sql = "UPDATE users SET password = ?, salt = ? WHERE id = ?";
        try {
            runner.execute(sql, hashedPassword, salt, id);
            LOG.info("Updated user password with id={}", id);
        } catch (SQLException ex) {
            LOG.error("Failed to update user with id={}", id, ex);
            throw new RuntimeException(ex);
        }
    }

    public record EloChangeSet(double winEloDiff, double loseEloDiff) {}

    public EloChangeSet updateStats(long winId, long loseId) {
        String sql = "CALL updateStats(?, ?, ?, ?)";

        Connection conn = null;
        CallableStatement stmt = null;
        try {
            conn = runner.getDataSource().getConnection();

            stmt = conn.prepareCall(sql);
            stmt.setLong(1, winId);
            stmt.setLong(2, loseId);
            stmt.registerOutParameter(3, Types.NUMERIC);
            stmt.registerOutParameter(4, Types.NUMERIC);

            stmt.execute();

            double winEloDiff = stmt.getBigDecimal(3).doubleValue();
            double loseEloDiff = stmt.getBigDecimal(4).doubleValue();

            LOG.info("Updated stats: winId={} loseId={}, winEloDiff={}, loseEloDiff={}", winId, loseId, winEloDiff, loseEloDiff);
            return new EloChangeSet(winEloDiff, loseEloDiff);
        } catch (SQLException ex) {
            LOG.error("Failed to update stats for winId={}, loseId={}", winId, loseId, ex);
            DbUtils.rollbackAndCloseQuietly(conn);
            throw new RuntimeException(ex);
        } finally {
            DbUtils.closeQuietly(conn);
            DbUtils.closeQuietly(stmt);
        }
    }

    public UserEntity getById(long id) {
        String sql = """
            SELECT id, username, country, elo, highestElo, wins, losses, bio, joinedOn
            FROM users
            WHERE id = ?
            """;
        try {
            UserEntity user = runner.query(sql, USER_MAPPER, id);
            LOG.info("Selected user={} by id={}", user, id);
            return user;
        } catch (SQLException ex) {
            LOG.error("Failed to fetch user by id={}", id, ex);
            throw new RuntimeException(ex);
        }
    }

    public List<UserEntity> getByRankedUsers(List<RankedEntity> users) {
        return getByIds(users.stream().map(RankedEntity::getId).toArray(Long[]::new));
    }

    public List<UserEntity> getByIds(Long[] ids) {
        String sql = """
            SELECT id, username, country, elo, wins, losses
            FROM users
            WHERE id = ANY (?)
            """;

        String idsStr = Arrays.stream(ids).map(Object::toString).collect(Collectors.joining(",", "[", "]"));

        Connection conn = null;
        PreparedStatement stmt = null;
        ResultSet rs = null;
        try {
            conn = runner.getDataSource().getConnection();
            stmt = conn.prepareStatement(sql);

            stmt.setArray(1, conn.createArrayOf("INTEGER", ids));
            rs = stmt.executeQuery();

            List<UserEntity> users = USER_LIST_MAPPER.handle(rs);
            LOG.info("Selected users={} by ids={}", users, idsStr);
            return users;
        } catch (SQLException e) {
            LOG.error("Failed to select users by ids={}", idsStr);
            throw new RuntimeException(e);
        } finally {
            DbUtils.closeQuietly(conn);
            DbUtils.closeQuietly(stmt);
            DbUtils.closeQuietly(rs);
        }
    }

    public List<UserEntity> getAll() {
        String sql = "SELECT id, username, country, elo, wins, losses FROM users";
        try {
            List<UserEntity> users = runner.query(sql, USER_LIST_MAPPER);
            LOG.info("Selected ALL records={} from the user table", users);
            return users;
        } catch (SQLException ex) {
            LOG.error("Failed to select ALL records from the user table");
            throw new RuntimeException(ex);
        }
    }

    public List<UserEntity> getLeaderboard(int page, int perPage) {
        String sql = """
            SELECT id, username, country, elo, wins, losses
            FROM users
            ORDER BY elo DESC LIMIT ? OFFSET ?
            """;

        page = Math.max(page, 1);
        int offset = (page - 1) * perPage;

        try {
            List<UserEntity> users = runner.query(sql, USER_LIST_MAPPER, perPage, offset);
            for (int i = 0; i < users.size(); i++) {
                int rank = (page - 1) * perPage + i + 1;
                users.get(i).setRank(rank);
            }
            LOG.info("Selected leaderboard={} for page={}, perPage={}", users, page, perPage);
            return users;
        } catch (SQLException ex) {
            LOG.error("Failed to select leaderboard for page={}, perPage={}", page, perPage);
            throw new RuntimeException(ex);
        }
    }

    public List<UserEntity> searchByName(String name, int page, int perPage) {
        String sql = """
            SELECT id, username, country, elo, wins, losses, (username <-> ?) as rank
            FROM users
            WHERE 1 = 1 AND username % ?
            ORDER BY rank DESC LIMIT ? OFFSET ?
            """;

        page = Math.max(page, 1);
        int offset = (page - 1) * perPage;

        try {
            List<UserEntity> users = runner.query(sql, USER_LIST_MAPPER, name, name, perPage, offset);
            for (int i = 0; i < users.size(); i++) {
                int rank = (page - 1) * perPage + i + 1;
                users.get(i).setRank(rank);
            }

            LOG.info("Selected users={} by name for name={}, page={}, perPage={}", users, name, page, perPage);
            return users;
        } catch (SQLException ex) {
            LOG.error("Failed to select users by name for name={}, page={}, perPage={}", name, page, perPage);
            throw new RuntimeException(ex);
        }
    }
}