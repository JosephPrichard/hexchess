package services;

import models.Challenge;
import org.apache.commons.dbutils.QueryRunner;
import org.apache.commons.dbutils.ResultSetHandler;
import org.apache.commons.dbutils.handlers.BeanListHandler;

import javax.sql.DataSource;
import java.sql.SQLException;
import java.sql.Timestamp;
import java.time.Duration;
import java.util.List;

import static utils.Globals.LOGGER;

public class ChallengeDao {

    public static final Duration THRESHOLD_EXPIRATION = Duration.ofDays(7);
    private static final ResultSetHandler<List<Challenge>> CHAL_LIST_MAPPER = new BeanListHandler<>(Challenge.class);

    public static class ChallengeException extends RuntimeException {
        public ChallengeException(String message) {
            super(message);
        }
    }

    private final QueryRunner runner;

    public ChallengeDao(DataSource ds) {
        runner = new QueryRunner(ds);
    }

    public void insertStatus(String challengeeId, String challengerId) throws ChallengeException {
         var sql = """
            BEGIN;
            INSERT INTO challenges (challengerId, challengeeId, status) VALUES (?, ?, ?);
            END""";
        try {
            runner.execute(sql, challengerId, challengeeId, Challenge.PENDING);
            LOGGER.info("Inserted a challenge for a challenge=[{},{}]", challengerId, challengeeId);
        } catch (SQLException ex) {
            LOGGER.error("Failed to insert a challenge for a challenge=[{},{}]", challengerId, challengeeId);
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
            END""";
        try {
            runner.execute(sql, status, challengeeId, challengerId);
            LOGGER.info("Updated the status={} for a challenge=[{},{}]", status, challengerId, challengeeId);
        } catch (SQLException ex) {
            LOGGER.error("Failed to update the status={} for a challenge=[{},{}]", status, challengerId, challengeeId);
            throw new RuntimeException(ex);
        }
    }

    public List<Challenge> getByChallengee(String challengeeId) {
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
            WHERE challengeeId = ?""";
        try {
            var challenges = runner.query(sql, CHAL_LIST_MAPPER, challengeeId);
            LOGGER.info("Selected challenges={} for challengee={}", challenges, challengeeId);
            return challenges;
        } catch (SQLException ex) {
            LOGGER.info("Failed to select challenges for challengee={}", challengeeId);
            throw new RuntimeException(ex);
        }
    }

    public void deleteExpired(String challengeeId, Duration threshold) throws ChallengeException {
        var timestamp = new Timestamp(System.currentTimeMillis() - threshold.toMillis());

        var sql = """
            BEGIN;
            DELETE FROM challenges WHERE challengeeId = ? AND madeOn < ?;
            END""";
        try {
            runner.execute(sql, challengeeId, timestamp);
            LOGGER.info("Deleted expired challenges for challengee={}", challengeeId);
        } catch (SQLException ex) {
            LOGGER.info("Failed to delete expired challenges for challengee={}", challengeeId);
            throw new RuntimeException(ex);
        }
    }

    public void deleteExpired(String challengeeId) throws ChallengeException {
        deleteExpired(challengeeId, THRESHOLD_EXPIRATION);
    }
}
