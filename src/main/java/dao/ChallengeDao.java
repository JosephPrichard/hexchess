package dao;

import models.Challenge;
import org.apache.commons.dbutils.DbUtils;
import org.apache.commons.dbutils.QueryRunner;
import org.apache.commons.dbutils.ResultSetHandler;
import org.apache.commons.dbutils.handlers.BeanListHandler;

import javax.sql.DataSource;
import java.sql.*;
import java.time.Duration;
import java.util.List;

import static utils.Globals.LOGGER;

public class ChallengeDao {

    public static final Duration THRESHOLD_EXPIRATION = Duration.ofDays(7);
    private static final ResultSetHandler<List<Challenge>> CHAL_LIST_MAPPER = new BeanListHandler<>(Challenge.class);

    private final QueryRunner runner;

    public ChallengeDao(DataSource ds) {
        runner = new QueryRunner(ds);
    }

    public static class ChallengeException extends RuntimeException {
        public ChallengeException(String message) {
            super(message);
        }
    }

    public void insert(String challengeeId, String challengerId) throws ChallengeException {
         var sql = """
             BEGIN;
                 INSERT INTO challenges (challengerId, challengeeId, status) VALUES (?, ?, ?);
             END
             """;
        try {
            runner.execute(sql, challengerId, challengeeId, Challenge.PENDING);
            LOGGER.info("Inserted a challenge=[{},{}]", challengerId, challengeeId);
        } catch (SQLException ex) {
            var nextEx = ex.getNextException();
            if ("23505".equals(nextEx.getSQLState())) {
                LOGGER.warn("Already made challenge=[{},{}]", challengerId, challengeeId, ex);
                throw new ChallengeDao.ChallengeException("Challenge has already been made against this user");
            }

            LOGGER.error("Failed to insert a challenge=[{},{}]", challengerId, challengeeId);
            throw new RuntimeException(ex);
        }
    }

    public void updateStatus(String challengeeId, String challengerId, int status) throws ChallengeException {
        if (status != Challenge.ACCEPTED && status != Challenge.REJECTED && status != Challenge.PENDING) {
            LOGGER.info("Failed to update status for challenge with challenger={} challengee={}, status is invalid", challengerId, challengeeId);
            throw new ChallengeException("Invalid status for challenge");
        }

        var sql = """
            BEGIN;
                UPDATE challenges SET status = ? WHERE challengeeId = ? AND challengerId = ?;
            END
            """;
        try {
            runner.execute(sql, status, challengeeId, challengerId);
            LOGGER.info("Updated the status={} for a challenge=[{},{}]", status,  challengerId, challengeeId);
        } catch (SQLException ex) {
            LOGGER.error("Failed to update the status={} for a challenge=[{},{}]", status, challengerId, challengeeId);
            throw new RuntimeException(ex);
        }
    }

    public List<Challenge> getByParticipant(String challengerId, String challengeeId, Duration threshold) {
        var timestamp = new Timestamp(System.currentTimeMillis() - threshold.toMillis());

        var sql = """
            SELECT
                c1.challengeeId,
                u1.username as challengeeUsername,
                u1.country as challengeeCountry,
                u1.elo as challengeeElo,
                c1.challengerId,
                u2.username as challengerUsername,
                u2.country as challengerCountry,
                u2.elo as challengerElo,
                c1.status,
                c1.madeOn
            FROM challenges c1
            INNER JOIN users as u1 ON u1.id = c1.challengeeId
            INNER JOIN users as u2 ON u2.id = c1.challengerId
            WHERE challengerId = COALESCE(?, challengerId) AND challengeeId = COALESCE(?, challengeeId) AND madeOn >= ?
            """;

        Connection conn = null;
        PreparedStatement stmt = null;
        ResultSet rs = null;
        try {
            conn = runner.getDataSource().getConnection();
            stmt = conn.prepareStatement(sql);

            stmt.setString(1, challengerId);
            stmt.setString(2, challengeeId);
            stmt.setTimestamp(3, timestamp);
            rs = stmt.executeQuery();

            var challenges = CHAL_LIST_MAPPER.handle(rs);
            LOGGER.info("Selected challenges={} for challenged={}", challenges, challengerId);
            return challenges;
        } catch (SQLException ex) {
            LOGGER.info("Failed to select challenges for challenged={}", challengerId);
            throw new RuntimeException(ex);
        } finally {
            DbUtils.closeQuietly(conn);
            DbUtils.closeQuietly(stmt);
            DbUtils.closeQuietly(rs);
        }
    }

    public List<Challenge> getByParticipant(String challengerId, String challengeeId) {
        return getByParticipant(challengerId, challengeeId, THRESHOLD_EXPIRATION);
    }

    public void deleteExpired(String userId, Duration threshold) throws ChallengeException {
        var timestamp = new Timestamp(System.currentTimeMillis() - threshold.toMillis());

        var sql = """
            BEGIN;
                DELETE FROM challenges WHERE (challengeeId = ? OR challengerId = ?) AND madeOn < ?;
            END
            """;
        try {
            runner.execute(sql, userId, userId, timestamp);
            LOGGER.info("Deleted expired challenges for userId={}", userId);
        } catch (SQLException ex) {
            LOGGER.info("Failed to delete expired challenges for userId={}", userId);
            throw new RuntimeException(ex);
        }
    }

    public void deleteExpired(String challengeeId) throws ChallengeException {
        deleteExpired(challengeeId, THRESHOLD_EXPIRATION);
    }
}
