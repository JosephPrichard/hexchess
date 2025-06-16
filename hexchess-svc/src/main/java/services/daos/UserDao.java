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
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ExecutionException;
import java.util.stream.Collectors;

import static utils.Globals.CPU_EXECUTOR;
import static utils.Globals.LOG;

public class UserDao {

    private static final ResultSetHandler<UserEntity> USER_MAPPER = new BeanHandler<>(UserEntity.class);
    private static final ResultSetHandler<List<UserEntity>> USER_LIST_MAPPER = new BeanListHandler<>(UserEntity.class);
    private static final ResultSetHandler<VerifiedUser> VERIFIED_USER_MAPPER = new BeanHandler<>(VerifiedUser.class);
    private static final ResultSetHandler<List<EloResult>> ELO_LIST_MAPPER = new BeanListHandler<>(EloResult.class);
    private static final ResultSetHandler<List<Long>> IDS_MAPPER = new BeanListHandler<>(Long.class);

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

    public record HashResult(String salt, String hashedPassword) {}

    public static HashResult generateHash(String password) {
        try {
            byte[] bytes = new byte[16];
            SecureRandom.getInstanceStrong().nextBytes(bytes);
            String salt = Base64.getEncoder().encodeToString(bytes);

            String saltedPassword = password + salt;
            String hashedPassword = BCrypt.withDefaults().hashToString(12, saltedPassword.toCharArray());
            return new HashResult(salt, hashedPassword);
        } catch (NoSuchAlgorithmException ex) {
            LOG.error("Error occurred while generated salt and hashed passwords", ex);
            throw new RuntimeException(ex);
        }
    }

    public UserEntity insert(UserInst inst) throws TakenUsernameException {
        HashResult hash = generateHash(inst.password());

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
                hash.hashedPassword(),
                hash.salt());
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

    public void batchInsert(List<UserInst> insts) throws TakenUsernameException {
        LOG.info("Starting batch insert for user insts={}", insts);

        if (insts.isEmpty()) {
            LOG.info("Finished batch insert, no insts were provided, this is a no-op");
            return;
        }

        List<CompletableFuture<HashResult>> hashFuts = insts.stream()
            .map((inst) -> CompletableFuture.supplyAsync(() -> generateHash(inst.password()), CPU_EXECUTOR))
            .toList();

        String sql = """
            INSERT INTO users (username, country, elo, highestElo, wins, losses, password, salt)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?) RETURNING id, username, country, elo, highestElo, wins, losses, bio;
            """;
        try {
            Object[][] params = new Object[insts.size()][];
            for (int i = 0; i < insts.size(); i++) {
                UserInst inst = insts.get(i);
                HashResult hash = hashFuts.get(i).get();

                Object[] row = new Object[8];
                row[0] = inst.username();
                row[1] = inst.country();
                row[2] = inst.elo();
                row[3] = inst.elo();
                row[4] = inst.wins();
                row[5] = inst.losses();
                row[6] = hash.hashedPassword();
                row[7] = hash.salt();

                params[i] = row;
            }

            int[] userIds = runner.batch(sql, params);
            LOG.info("Finished batch insert for new users with ids={}", userIds);
        } catch (SQLException | InterruptedException | ExecutionException ex) {
            LOG.error("Failed to perform batch insert on users={}", insts, ex);
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
        } catch (Exception ex) {
            LOG.error("Failed to select user credentials for user={}", username, ex);
            DbUtils.rollbackQuietly(conn);
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
        HashResult hash = generateHash(newPassword);

        String sql = "UPDATE users SET password = ?, salt = ? WHERE id = ?";
        try {
            runner.execute(sql, hash.hashedPassword(), hash.salt(), id);
            LOG.info("Updated user password with id={}", id);
        } catch (SQLException ex) {
            LOG.error("Failed to update user with id={}", id, ex);
            throw new RuntimeException(ex);
        }
    }

    public record EloChangeSet(double winEloDiff, double loseEloDiff) {}

    public double probabilityWins(double elo1, double elo2) {
        return 1.0 / (1.0 + Math.pow(10, (elo1 - elo2) / 400.0));
    }

    public EloChangeSet updateStats(long winId, long loseId) {
        String getEloSql = "SELECT elo FROM users WHERE id = ?";
        String updateWinsSql = """
            UPDATE users
            SET elo = ?, wins = wins + 1, highestElo = GREATEST(highestElo, ?)
            WHERE id = ?;
            """;
        String updateLossSql = """
            UPDATE users
            SET elo = ?, losses = losses + 1
            WHERE id = ?
            """;

        Connection conn = null;
        PreparedStatement getEloStmt = null;
        PreparedStatement updateWinsStmt = null;
        PreparedStatement updateLossStmt = null;
        ResultSet getWinRs = null;
        ResultSet getLossRs = null;
        try {
            conn = runner.getDataSource().getConnection();
            conn.setAutoCommit(false);
            conn.setTransactionIsolation(Connection.TRANSACTION_SERIALIZABLE);

            getEloStmt = conn.prepareStatement(getEloSql);
            updateWinsStmt = conn.prepareStatement(updateWinsSql);
            updateLossStmt = conn.prepareStatement(updateLossSql);

            getEloStmt.setLong(1, winId);
            getWinRs = getEloStmt.executeQuery();
            if (!getWinRs.next()) {
                LOG.warn("Couldn't find winner with id={}", winId);
                return null;
            }
            double winElo = getWinRs.getDouble("elo");

            getEloStmt.setLong(1, loseId);
            getLossRs = getEloStmt.executeQuery();
            if (!getLossRs.next()) {
                LOG.warn("Couldn't find loser with id={}", loseId);
                return null;
            }
            double loseElo = getLossRs.getDouble("elo");

            double winEloNext = winElo + (30 * (1 - probabilityWins(loseElo, winElo)));
            updateWinsStmt.setDouble(1, winEloNext);
            updateWinsStmt.setDouble(2, winEloNext);
            updateWinsStmt.setLong(3, winId);
            updateWinsStmt.executeUpdate();

            double loseEloNext = loseElo + ((30 * probabilityWins(winElo, loseElo)) * -1);
            updateLossStmt.setDouble(1, loseEloNext);
            updateLossStmt.setLong(2, loseId);
            updateLossStmt.executeUpdate();

            conn.commit();

            double winEloDiff = winEloNext - winElo;
            double loseEloDiff = loseEloNext - loseElo;

            LOG.info("Updated stats: winId={} loseId={}, winEloDiff={}, loseEloDiff={}", winId, loseId, winEloDiff, loseEloDiff);
            return new EloChangeSet(winEloDiff, loseEloDiff);
        } catch (Exception ex) {
            LOG.error("Failed to update stats for winId={}, loseId={}", winId, loseId, ex);
            DbUtils.rollbackQuietly(conn);
            throw new RuntimeException(ex);
        } finally {
            DbUtils.closeQuietly(conn);
            DbUtils.closeQuietly(getEloStmt);
            DbUtils.closeQuietly(updateWinsStmt);
            DbUtils.closeQuietly(updateLossStmt);
            DbUtils.closeQuietly(getWinRs);
            DbUtils.closeQuietly(getLossRs);
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

            List<UserEntity> userList = USER_LIST_MAPPER.handle(rs);
            LOG.info("Selected users={} by ids={}", userList, idsStr);
            return userList;
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

    @Data
    @NoArgsConstructor
    @AllArgsConstructor
    public static class EloResult {
        private long id;
        private float elo;
    }

    public List<EloResult> getEloList(int afterId, int count) {
        String sql = "SELECT id, elo FROM users WHERE id > ? ORDER BY id LIMIT ?";

        try {
            List<EloResult> eloList = runner.query(sql, ELO_LIST_MAPPER, afterId, count);
            LOG.info("Selected eloList={} after id={}", eloList, afterId);
            return eloList;
        } catch (SQLException ex) {
            LOG.error("Failed to select eloList after id={}", afterId);
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