package infra;

import models.ChallengeEntity;
import org.apache.commons.dbutils.QueryRunner;

import javax.sql.DataSource;
import java.util.List;

import static utils.Globals.LOGGER;

public class ChallengeDao {

    public static class ChallengeException extends RuntimeException {
        public ChallengeException(String message) {
            super(message);
        }
    }

    private final QueryRunner runner;

    public ChallengeDao(DataSource ds) {
        runner = new QueryRunner(ds);
    }

    public List<ChallengeEntity> getByChallenged(String userId) {
        return null;
    }

    public void updateStatus(String challengerId, String challengeeId, int status) throws ChallengeException {
        if (status != ChallengeEntity.ACCEPTED && status != ChallengeEntity.REJECTED && status != ChallengeEntity.PENDING) {
            LOGGER.info("Failed to update status for challenge with challenger={} challengee={}, status is invalid", challengerId, challengeeId);
            throw new ChallengeException("Invalid status for challenge");
        }
    }
}
