package services.daos;

import lombok.*;
import models.entities.ReplayEntity;
import org.apache.commons.dbutils.DbUtils;
import org.apache.commons.dbutils.QueryRunner;
import org.apache.commons.dbutils.ResultSetHandler;
import org.apache.commons.dbutils.handlers.BeanHandler;
import org.apache.commons.dbutils.handlers.BeanListHandler;
import org.apache.commons.dbutils.handlers.ScalarHandler;

import javax.sql.DataSource;
import java.sql.Connection;
import java.sql.PreparedStatement;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.util.List;

import static utils.Globals.LOG;

public class ReplayDao {

    private static final ResultSetHandler<ReplayEntity> HIST_MAPPER = new BeanHandler<>(ReplayEntity.class);
    private static final ResultSetHandler<List<ReplayEntity>> HIST_LIST_MAPPER = new BeanListHandler<>(ReplayEntity.class);
    private static final ScalarHandler<Long> ID_MAPPER = new ScalarHandler<>("id");

    private final QueryRunner runner;

    public ReplayDao(DataSource ds) {
        runner = new QueryRunner(ds);
    }

    @With
    public record ReplayInst(long whiteId, long blackId, int result, int cause, double winElo, double loseElo, String moveListJson) {}

    public void insert(long whiteId, long blackId, int result, int cause, double winEloDiff, double loseEloDiff, String moveListJson) {
        insert(new ReplayInst(whiteId, blackId, result, cause, winEloDiff, loseEloDiff, moveListJson));
    }

    public void insert(ReplayInst inst) {
        String sql = "INSERT INTO replays (whiteId, blackId, result, cause, winElo, loseElo, moveList) VALUES (?, ?, ?, ?, ?, ?, ? :: JSONB) RETURNING id";
        try {
            Long id = runner.query(sql, ID_MAPPER,
                inst.whiteId(),
                inst.blackId(),
                inst.result(),
                inst.cause(),
                inst.winElo(),
                inst.loseElo(),
                inst.moveListJson());
            LOG.info("Inserted a replay, with id={} and inst={}", id, inst);
        } catch (SQLException ex) {
            LOG.error("Failed to insert a replay, inst={}", inst);
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
                r1.cause,
                r1.playedOn,
                r1.winElo,
                r1.loseElo,
                u1.username as whiteName,
                u1.country as whiteCountry,
                u1.elo as whiteElo,
                u2.username as blackName,
                u2.country as blackCountry,
                u2.elo as blackElo
            FROM replays as r1
            INNER JOIN users as u1 ON u1.id = r1.whiteId
            INNER JOIN users as u2 ON u2.id = r1.blackId
            WHERE r1.id = ?
            """;
        try {
            ReplayEntity replay = runner.query(sql, HIST_MAPPER, id);
            LOG.info("Selected replay={} for id={}", replay, id);
            return replay;
        } catch (SQLException ex) {
            LOG.error("Failed to select replay for id={}", id);
            throw new RuntimeException(ex);
        }
    }

    public String getReplayMoveList(long id) {
        String sql = "SELECT moveList :: JSON as moveListJson FROM replays WHERE id = ?";

        Connection conn = null;
        PreparedStatement stmt = null;
        ResultSet rs = null;
        try {
            conn = runner.getDataSource().getConnection();
            stmt = conn.prepareStatement(sql);
            stmt.setLong(1, id);

            rs = stmt.executeQuery();

            if (!rs.next()) {
                LOG.warn("No replay found with id={}", id);
                return null;
            }

            LOG.info("Selected replay moveList for id={}", id);
            return rs.getString("moveListJson");
        } catch (SQLException ex) {
            LOG.error("Failed to select replay for id={}", id);
            throw new RuntimeException(ex);
        } finally {
            DbUtils.closeQuietly(conn);
            DbUtils.closeQuietly(stmt);
            DbUtils.closeQuietly(rs);
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
                r1.cause,
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
            LOG.info("Selected user replays={} page for userId={}, afterId={}", replays, userId, afterId);
            return replays;
        } catch (SQLException ex) {
            LOG.info("Failed to select user replays page for userId={}, afterId={}", userId, afterId);
            throw new RuntimeException(ex);
        }
    }
}