package services;

import chess.*;
import models.common.ColorSelect;
import models.common.TimeControl;
import models.entities.ReplayEntity;
import models.state.ChessRoom;
import models.state.Player;
import org.junit.jupiter.api.Test;
import services.daos.DictionaryDao;
import services.daos.ReplayDao;
import services.daos.UserDao;
import services.game.GameService;

import java.util.List;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.*;

public class GameServiceTest {

    @Test
    void testJoinGame_JoinWhite() {
        // given
        DictionaryDao dictionaryDao = mock(DictionaryDao.class);
        GameService gameService = new GameService(dictionaryDao, null, null);

        String gameId = "abc123";
        ChessRoom room = ChessRoom.startWithGame(gameId, TimeControl.REAL_TIME);
        room.setFirstColor(ColorSelect.WHITE);
        Player player = new Player(1L, "name", "us", 0f);

        // when
        when(dictionaryDao.getRoom(gameId)).thenReturn(room);
        when(dictionaryDao.setRoom(any(), any())).thenReturn(room);

        ChessRoom updated = gameService.join(gameId, player);

        // then
        verify(dictionaryDao, times(1)).setRoom(eq(gameId), any());

        assertEquals(player, updated.getWhitePlayer());
    }

    @Test
    void testJoinGame_BothPlayersAlreadyExist() {
        // given
        DictionaryDao dictionaryDao = mock(DictionaryDao.class);
        GameService gameService = new GameService(dictionaryDao, null, null);

        String gameId = "abc123";
        Player white = new Player(1L, "name1", "us", 0f);
        Player black = new Player(2L, "name2", "us", 0f);

        ChessRoom room = ChessRoom.startWithGame(gameId, TimeControl.REAL_TIME);
        room.setWhitePlayer(white);
        room.setBlackPlayer(black);

        // when
        when(dictionaryDao.getRoom(gameId)).thenReturn(room);

        ChessRoom result = gameService.join(gameId, new Player(3L, "name3", "us", 0f));

        // then
        assertEquals(room, result); // No-op join
    }

    @Test
    void testMakeMove() {
        // given
        DictionaryDao dictionaryDao = mock(DictionaryDao.class);
        UserDao userDao = mock(UserDao.class);
        ReplayDao replayDao = mock(ReplayDao.class);
        GameService gameService = new GameService(dictionaryDao, userDao, replayDao);

        String gameId = "game123";
        Player white = new Player(1L, "name1", "us", 0f);
        Player black = new Player(2L, "name2", "us", 0f);
        Move move = new Move(Hexagon.of(0, 0), Hexagon.of(0, 1));
        byte piece = ChessBoard.BLACK_PAWN;
        PieceMove pm = new PieceMove(piece, move.getFrom(), move.getTo());

        ChessGame game = mock(ChessGame.class);
        ChessBoard board = mock(ChessBoard.class);

        ChessRoom room = spy(ChessRoom.startWithGame(gameId, TimeControl.REAL_TIME));
        room.setWhitePlayer(white);
        room.setBlackPlayer(black);
        room.setGame(game);

        // when
        doNothing().when(room).addMove(any());

        when(game.getBoard()).thenReturn(board);
        when(game.isValidMove(any())).thenReturn(true);
        when(game.makeMove(move)).thenReturn(pm);

        when(board.getPiece(any(Hexagon.class))).thenReturn(piece);
        when(board.isWhiteTurn()).thenReturn(true);

        when(dictionaryDao.getRoom(any())).thenReturn(room);
        when(dictionaryDao.setRoom(anyString(), any())).thenReturn(room);

        GameService.MakeMoveResult result = gameService.makeMove(gameId, white, move);

        // then
        verify(game).isValidMove(move);
        verify(room).addMove(pm);

        verify(dictionaryDao).getRoom(gameId);
        verify(dictionaryDao).setRoom(eq(gameId), any());

        assertNotNull(result);
        assertEquals(room.getId(), result.room().getId());
        assertEquals(pm, result.pm());
    }

    @Test
    void testOnFinishGame() {
        // given
        DictionaryDao dictionaryDao = mock(DictionaryDao.class);
        UserDao userDao = mock(UserDao.class);
        ReplayDao replayDao = mock(ReplayDao.class);
        GameService gameService = new GameService(dictionaryDao, userDao, replayDao);

        ChessRoom room = ChessRoom.startWithGame("gameId", TimeControl.REAL_TIME);
        Player white = new Player(1L, "name1", "us", 0f);
        Player black = new Player(2L, "name2", "us", 0f);

        room.setWhitePlayer(white);
        room.setBlackPlayer(black);
        room.setMoveList(List.of(new PieceMove((byte) 1, Hexagon.of(0, 0), Hexagon.of(0, 1))));

        // when
        when(userDao.updateStats(anyLong(), anyLong())).thenReturn(new UserDao.EloChangeSet(10, -10));

        gameService.onFinishGame(room, true, ReplayEntity.CHECKMATE);

        // then
        verify(dictionaryDao).incrLeaderboardUser(
            new DictionaryDao.EloChangeSet(1L, 10),
            new DictionaryDao.EloChangeSet(2L, -10));
        verify(replayDao).insert(eq(1L), eq(2L), eq(ReplayEntity.WHITE_WIN), eq(ReplayEntity.CHECKMATE), eq(10.0d), eq(-10.0d), anyString());
    }

    @Test
    void testForfeit_BlackForfeits() {
        // given
        DictionaryDao dictionaryDao = mock(DictionaryDao.class);
        UserDao userDao = mock(UserDao.class);
        ReplayDao replayDao = mock(ReplayDao.class);
        GameService gameService = new GameService(dictionaryDao, userDao, replayDao);

        Player white = new Player(1L, "name1", "us", 0f);
        Player black = new Player(2L, "name2", "us", 0f);
        ChessRoom room = ChessRoom.startWithGame("gameId", TimeControl.REAL_TIME);
        room.setWhitePlayer(white);
        room.setBlackPlayer(black);

        // when
        when(dictionaryDao.getRoom(anyString())).thenReturn(room);

        gameService.forfeit("gameId", black);

        // then
        assertTrue(room.isEnded());
        verify(dictionaryDao).getRoom("gameId");
        verify(dictionaryDao).setRoom(eq("gameId"), eq(room));
    }
}
