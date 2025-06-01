package services.daos;

import lombok.*;
import models.entities.ChallengeEntity;
import org.apache.commons.dbutils.DbUtils;
import org.apache.commons.dbutils.QueryRunner;
import org.apache.commons.dbutils.ResultSetHandler;
import org.apache.commons.dbutils.handlers.BeanHandler;
import org.apache.commons.dbutils.handlers.BeanListHandler;

import javax.sql.DataSource;
import java.sql.*;
import java.time.Duration;
import java.util.List;

import static utils.Globals.LOG;

public class ChallengeDao {

    public static final Duration THRESHOLD_EXPIRATION = Duration.ofDays(7);
    private static final ResultSetHandler<ChallengeEntity> CHAL_MAPPER = new BeanHandler<>(ChallengeEntity.class);
    private static final ResultSetHandler<List<ChallengeEntity>> CHAL_LIST_MAPPER = new BeanListHandler<>(ChallengeEntity.class);
    private static final ResultSetHandler<List<DeleteResult>> DELRES_MAPPER = new BeanListHandler<>(DeleteResult.class);

    private final QueryRunner runner;

    public ChallengeDao(DataSource ds) {
        runner = new QueryRunner(ds);
    }

    public static class DuplicateException extends RuntimeException {}

    public static class SelfException extends RuntimeException {}

    public static class ParticipantException extends RuntimeException {}

    public record ChallengeInst(long challengerId, long challengeeId, String timeControl, String startColor) { }

    public ChallengeEntity insert(ChallengeInst inst) {
        return insert(inst.challengerId(), inst.challengeeId(), inst.timeControl(), inst.startColor());
    }

    public ChallengeEntity insert(long challengerId, long challengeeId, String timeControl, String startColor) {
        return insert(challengerId, challengeeId, timeControl, startColor, new Timestamp(System.currentTimeMillis()));
    }

    public ChallengeEntity insert(long challengerId, long challengeeId, String timeControl, String startColor, Timestamp madeOn) {
        if (challengeeId == challengerId) {
            throw new ChallengeDao.SelfException();
        }

        String sql = """
            WITH inserted_challenges AS (
                INSERT INTO challenges (challengerId, challengeeId, timeControl, startColor, madeOn) VALUES (?, ?, ?, ?, ?) RETURNING *
            )
            SELECT
                c1.challengeeId,
                u1.username as challengeeName,
                u1.country as challengeeCountry,
                u1.elo as challengeeElo,
                c1.challengerId,
                u2.username as challengerName,
                u2.country as challengerCountry,
                u2.elo as challengerElo,
                c1.timeControl,
                c1.startColor,
                c1.madeOn
            FROM inserted_challenges c1
            INNER JOIN users as u1 ON u1.id = c1.challengeeId
            INNER JOIN users as u2 ON u2.id = c1.challengerId
            """;
        try {
            LOG.info("Inserting challenge with challengerId={}, challengeeId={}, timeControl={}, startColor={}", challengerId, challengeeId, timeControl, startColor);
            ChallengeEntity challenge = runner.query(sql, CHAL_MAPPER, challengerId, challengeeId, timeControl, startColor, madeOn);
            LOG.info("Inserted a challenge={}", challenge);
            return challenge;
        } catch (SQLException ex) {
            SQLException nextEx = ex.getNextException();

            switch (nextEx.getSQLState()) {
            case "23505":
                LOG.warn("Already made challenge=[challengerId={},challengeeId={}]", challengerId, challengeeId);
                throw new DuplicateException();
            case "23503", "23506":
                LOG.warn("Violating key constraint exception when inserting challenge=[challengerId={},challengeeId={}]", challengerId, challengeeId);
                throw new ParticipantException();
            default:
                LOG.error("Failed to insert a challenge=[challengerId={},challengeeId={}] with violation={}", challengerId, challengeeId, nextEx.getSQLState(), ex);
                throw new RuntimeException(ex);
            }
        }
    }

    @Data
    @AllArgsConstructor
    @NoArgsConstructor
    public static class DeleteResult {
        private long challengerId;
        private long challengeeId;
        private String timeControl;
        private String startColor;
    }

    public DeleteResult delete(long challengerId, long challengeeId) {
        String sql = """
            DELETE FROM challenges WHERE challengerId = ? AND challengeeId = ?
            RETURNING challengerid, challengeeid, timeControl, startColor;
            """;

        try {
            List<DeleteResult> results = runner.query(sql, DELRES_MAPPER, challengerId, challengeeId);
            LOG.info("Delete challenge=[{},{}], deleting {} rows", challengerId, challengeeId, results);
            return results.isEmpty() ? null : results.getFirst();
        } catch (SQLException ex) {
            LOG.error("Failed to delete a challenge=[{},{}]", challengerId, challengeeId, ex);
            throw new RuntimeException(ex);
        }
    }

    public List<ChallengeEntity> getByParticipant(Long challengerId, Long challengeeId, Duration threshold) {
        Timestamp timestamp = new Timestamp(System.currentTimeMillis() - threshold.toMillis());

        String sql = """
            SELECT
                c1.challengeeId,
                u1.username as challengeeName,
                u1.country as challengeeCountry,
                u1.elo as challengeeElo,
                c1.challengerId,
                u2.username as challengerName,
                u2.country as challengerCountry,
                u2.elo as challengerElo,
                c1.timeControl,
                c1.startColor,
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
            LOG.info("Selected challenges={} for challengerId={}, challengeeId={}", challenges, challengerId, challengeeId);
            return challenges;
        } catch (SQLException ex) {
            LOG.error("Failed to select challenges for challenged={}", challengerId, ex);
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
            LOG.info("Deleted {} expired challenges for userId={}", rows, userId);
        } catch (SQLException ex) {
            LOG.error("Failed to delete expired challenges for userId={}", userId, ex);
            throw new RuntimeException(ex);
        }
    }

    public void deleteExpired(long challengeeId) {
        deleteExpired(challengeeId, THRESHOLD_EXPIRATION);
    }
}
