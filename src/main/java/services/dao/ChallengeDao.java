package services.dao;

import models.ChallengeEntity;
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
    private static final ResultSetHandler<List<ChallengeEntity>> CHAL_LIST_MAPPER = new BeanListHandler<>(ChallengeEntity.class);

    private final QueryRunner runner;

    public ChallengeDao(DataSource ds) {
        runner = new QueryRunner(ds);
    }

    public static class DuplicateException extends RuntimeException {}

    public static class SelfException extends RuntimeException {}

    public static class ParticipantException extends RuntimeException {}

    public void insert(long challengerId, long challengeeId) {
        insert(challengerId, challengeeId, new Timestamp(System.currentTimeMillis()));
    }

    public void insert(long challengerId, long challengeeId, Timestamp madeOn) {
        if (challengeeId == challengerId) {
            throw new ChallengeDao.SelfException();
        }

        String sql = "INSERT INTO challenges (challengerId, challengeeId, madeOn) VALUES (?, ?, ?)";
        try {
            runner.execute(sql, challengerId, challengeeId, madeOn);
            LOGGER.info("Inserted a challenge=[challengerId={},challengeeId={}]", challengerId, challengeeId);
        } catch (SQLException ex) {
            SQLException nextEx = ex.getNextException();

            switch (nextEx.getSQLState()) {
            case "23505":
                LOGGER.warn("Already made challenge=[challengerId={},challengeeId={}]", challengerId, challengeeId);
                throw new ChallengeDao.DuplicateException();
            case "23506":
                LOGGER.warn("Violating key constraint exception when inserting challenge=[challengerId={},challengeeId={}]", challengerId, challengeeId);
                throw new ParticipantException();
            default:
                LOGGER.error("Failed to insert a challenge=[challengerId={},challengeeId={}]", challengerId, challengeeId, ex);
                throw new RuntimeException(ex);
            }
        }
    }

    public int delete(long challengerId, long challengeeId) {
        String sql = "DELETE FROM challenges WHERE challengerId = ? AND challengeeId = ?";

        try {
            int count = runner.update(sql, challengerId, challengeeId);
            LOGGER.info("Delete challenge=[{},{}], deleting {} rows", challengerId, challengeeId, count);
            return count;
        } catch (SQLException ex) {
            LOGGER.error("Failed to delete a challenge=[{},{}]", challengerId, challengeeId, ex);
            throw new RuntimeException(ex);
        }
    }

    public List<ChallengeEntity> getByParticipant(Long challengerId, Long challengeeId, Duration threshold) {
        Timestamp timestamp = new Timestamp(System.currentTimeMillis() - threshold.toMillis());

        String sql = """
            SELECT
                c1.challengeeId,
                u1.username as challengeeName,
                u1.elo as challengeeElo,
                c1.challengerId,
                u2.username as challengerName,
                u2.elo as challengerElo,
                c1.madeOn
            FROM challenges c1
            INNER JOIN users as u1 ON u1.id = c1.challengeeId
            INNER JOIN users as u2 ON u2.id = c1.challengerId
            WHERE challengerId = COALESCE(?, challengerId)
              AND challengeeId = COALESCE(?, challengeeId)
              AND madeOn >= ?
            ORDER BY madeOn DESC
            """;

        Connection conn = null;
        PreparedStatement stmt = null;
        ResultSet rs = null;
        try {
            conn = runner.getDataSource().getConnection();
            stmt = conn.prepareStatement(sql);

            if (challengerId != null) {
                stmt.setLong(1, challengerId);
            } else {
                stmt.setNull(1, Types.INTEGER);
            }
            if (challengeeId != null) {
                stmt.setLong(2, challengeeId);
            } else {
                stmt.setNull(2, Types.INTEGER);
            }
            stmt.setTimestamp(3, timestamp);
            rs = stmt.executeQuery();

            List<ChallengeEntity> challenges = CHAL_LIST_MAPPER.handle(rs);
            LOGGER.info("Selected challenges={} for challenged={}", challenges, challengerId);
            return challenges;
        } catch (SQLException ex) {
            LOGGER.error("Failed to select challenges for challenged={}", challengerId, ex);
            throw new RuntimeException(ex);
        } finally {
            DbUtils.closeQuietly(conn);
            DbUtils.closeQuietly(stmt);
            DbUtils.closeQuietly(rs);
        }
    }

    public List<ChallengeEntity> getByParticipant(Long challengerId, Long challengeeId) {
        return getByParticipant(challengerId, challengeeId, THRESHOLD_EXPIRATION);
    }

    public void deleteExpired(long userId, Duration threshold) {
        Timestamp timestamp = new Timestamp(System.currentTimeMillis() - threshold.toMillis());

        String sql = "DELETE FROM challenges WHERE (challengeeId = ? OR challengerId = ?) AND madeOn < ?";
        try {
            int rows = runner.update(sql, userId, userId, timestamp);
            LOGGER.info("Deleted {} expired challenges for userId={}", rows, userId);
        } catch (SQLException ex) {
            LOGGER.error("Failed to delete expired challenges for userId={}", userId, ex);
            throw new RuntimeException(ex);
        }
    }

    public void deleteExpired(long challengeeId) {
        deleteExpired(challengeeId, THRESHOLD_EXPIRATION);
    }
}
