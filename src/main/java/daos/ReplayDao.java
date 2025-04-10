package daos;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.ToString;
import models.ReplayEntity;
import org.apache.commons.dbutils.QueryRunner;
import org.apache.commons.dbutils.ResultSetHandler;
import org.apache.commons.dbutils.handlers.BeanHandler;
import org.apache.commons.dbutils.handlers.BeanListHandler;

import javax.sql.DataSource;
import java.sql.SQLException;
import java.util.List;

import static utils.Globals.LOGGER;

public class ReplayDao {

    private static final ResultSetHandler<ReplayEntity> HIST_MAPPER = new BeanHandler<>(ReplayEntity.class);
    private static final ResultSetHandler<List<ReplayEntity>> HIST_LIST_MAPPER = new BeanListHandler<>(ReplayEntity.class);

    private final QueryRunner runner;

    public ReplayDao(DataSource ds) {
        runner = new QueryRunner(ds);
    }

    @Data
    @AllArgsConstructor
    public static class ReplayInst {
        public long whiteId;
        public long blackId;
        public int result;
        public Double winEloDiff;
        public Double loseEloDiff;
        @ToString.Exclude
        public String moveListJson;
    }

    public void insert(long whiteId, long blackId, int result, double winEloDiff, double loseEloDiff, String moveListJson) {
        insert(new ReplayInst(whiteId, blackId, result, winEloDiff, loseEloDiff, moveListJson));
    }

    public void insert(ReplayInst replayInst) {
        String sql = "INSERT INTO replays (whiteId, blackId, result, winElo, loseElo, moveList) VALUES (?, ?, ?, ?, ?, ? :: JSONB)";
        try {
            runner.execute(sql,
                replayInst.getWhiteId(),
                replayInst.getBlackId(),
                replayInst.getResult(),
                replayInst.getWinEloDiff(),
                replayInst.getLoseEloDiff(),
                replayInst.getMoveListJson());
            LOGGER.info("Inserted a replay={}", replayInst);
        } catch (SQLException ex) {
            LOGGER.error("Failed to insert a replay={}", replayInst);
            throw new RuntimeException(ex);
        }
    }

    public ReplayEntity getReplay(long id) {
        String sql = """
            SELECT
                r1.id,
                r1.whiteId,
                r1.blackId,
                r1.result,
                r1.playedOn,
                r1.winElo,
                r1.loseElo,
                u1.username as whiteName,
                u1.country as whiteCountry,
                u1.elo as whiteElo,
                u2.username as blackName,
                u2.country as blackCountry,
                u2.elo as blackElo,
                r1.moveList :: JSON as moveListJson
            FROM replays as r1
            INNER JOIN users as u1 ON u1.id = r1.whiteId
            INNER JOIN users as u2 ON u2.id = r1.blackId
            WHERE r1.id = ?
            """;
        try {
            ReplayEntity replay = runner.query(sql, HIST_MAPPER, id);
            LOGGER.info("Selected replay={} for id={}", replay, id);
            return replay;
        } catch (SQLException ex) {
            LOGGER.error("Failed to select replay for id={}", id);
            throw new RuntimeException(ex);
        }
    }

    public List<ReplayEntity> getUserReplays(Long userId, Long afterId, int perPage) {
        if (userId == null) {
            throw new RuntimeException("Expected userId to be non null");
        }
        if (afterId == null) {
            afterId = Long.MAX_VALUE;
        }

        String sql = """
            SELECT
                r1.id,
                r1.whiteId,
                r1.blackId,
                r1.result,
                r1.playedOn,
                r1.winElo,
                r1.loseElo,
                u1.username as whiteName,
                u1.country as whiteCountry,
                u2.username as blackName,
                u2.country as blackCountry
            FROM replays as r1
            INNER JOIN users as u1 ON u1.id = r1.whiteId
            INNER JOIN users as u2 ON u2.id = r1.blackId
            WHERE r1.id < ?
                AND (r1.whiteId = ? OR r1.blackId = ?)
            ORDER BY r1.id DESC LIMIT ?
            """;
        try {
            List<ReplayEntity> replays = runner.query(sql, HIST_LIST_MAPPER, afterId, userId, userId, perPage);
            LOGGER.info("Selected user replays={} page for userId={}, afterId={}", replays, userId, afterId);
            return replays;
        } catch (SQLException ex) {
            LOGGER.info("Failed to select user replays page for userId={}, afterId={}", userId, afterId);
            throw new RuntimeException(ex);
        }
    }
}