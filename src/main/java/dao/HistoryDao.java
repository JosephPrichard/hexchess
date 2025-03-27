package dao;

import lombok.AllArgsConstructor;
import lombok.Data;
import models.History;
import org.apache.commons.dbutils.QueryRunner;
import org.apache.commons.dbutils.ResultSetHandler;
import org.apache.commons.dbutils.handlers.BeanHandler;
import org.apache.commons.dbutils.handlers.BeanListHandler;

import javax.sql.DataSource;
import java.sql.SQLException;
import java.util.List;

import static utils.Globals.LOGGER;

public class HistoryDao {

    private static final ResultSetHandler<History> HIST_MAPPER = new BeanHandler<>(History.class);
    private static final ResultSetHandler<List<History>> HIST_LIST_MAPPER = new BeanListHandler<>(History.class);

    private final QueryRunner runner;

    public HistoryDao(DataSource ds) {
        runner = new QueryRunner(ds);
    }

    @Data
    @AllArgsConstructor
    public static class HistoryInst {
        public String whiteId;
        public String blackId;
        public int result;
        public Double winEloDiff;
        public Double loseEloDiff;
        public String data;
    }

    public void insert(String whiteId, String blackId, int result, double winEloDiff, double loseEloDiff, String data) {
        insert(new HistoryInst(whiteId, blackId, result, winEloDiff, loseEloDiff, data));
    }

    public void insert(HistoryInst historyInst) {
        String sql = "INSERT INTO game_histories (whiteId, blackId, result, data, winElo, loseElo) VALUES (?, ?, ?, ? ::json, ?, ?)";
        try {
            runner.execute(sql, historyInst.whiteId, historyInst.blackId,
                historyInst.result, historyInst.data, historyInst.winEloDiff, historyInst.loseEloDiff);
            LOGGER.info("Inserted a history={}", historyInst);
        } catch (SQLException ex) {
            LOGGER.error("Failed to insert a history={}", historyInst);
            throw new RuntimeException(ex);
        }
    }

    public History getHistory(long id) {
        String sql = """
            SELECT
                h1.id,
                h1.whiteId,
                h1.blackId,
                h1.result,
                h1.data,
                h1.playedOn,
                h1.winElo,
                h1.loseElo,
                u1.username as whiteName,
                u2.username as blackName,
                u1.country as whiteCountry,
                u2.country as blackCountry
            FROM game_histories as h1
            INNER JOIN users as u1 ON u1.id = h1.whiteId
            INNER JOIN users as u2 ON u2.id = h1.blackId
            WHERE h1.id = ?
            """;
        try {
            History history = runner.query(sql, HIST_MAPPER, id);
            LOGGER.info("Selected history={} for id={}", history, id);
            return history;
        } catch (SQLException ex) {
            LOGGER.error("Failed to select history for id={}", id);
            throw new RuntimeException(ex);
        }
    }

    public List<History> getUserHistories(String userId, Long afterId, int perPage) {
        if (userId == null) {
            throw new RuntimeException("Expected userId to be non null");
        }
        if (afterId == null) {
            afterId = Long.MAX_VALUE;
        }

        String sql = """
            SELECT
                h1.id,
                h1.whiteId,
                h1.blackId, h1.result,
                h1.playedOn,
                h1.winElo,
                h1.loseElo,
                u1.username as whiteName,
                u2.username as blackName,
                u1.country as whiteCountry,
                u2.country as blackCountry
            FROM game_histories as h1
            INNER JOIN users as u1 ON u1.id = h1.whiteId
            INNER JOIN users as u2 ON u2.id = h1.blackId
            WHERE h1.id < ?
                AND (h1.whiteId = ? OR h1.blackId = ?)
            ORDER BY h1.id DESC LIMIT ?
            """;

        try {
            List<History> histories = runner.query(sql, HIST_LIST_MAPPER, afterId, userId, userId, perPage);
            LOGGER.info("Selected user histories={} page for userId={}, afterId={}", histories, userId, afterId);
            return histories;
        } catch (SQLException ex) {
            LOGGER.info("Failed to select user histories page for userId={}, afterId={}", userId, afterId);
            throw new RuntimeException(ex);
        }
    }
}