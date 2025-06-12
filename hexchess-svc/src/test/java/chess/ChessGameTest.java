package chess;

import org.apache.commons.collections.CollectionUtils;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.Test;

import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;

import static chess.ChessBoard.*;
import static utils.Globals.LOG;

public class ChessGameTest {

    @Test
    public void testHexagonToString() {
        String str;

        str = new Hexagon(5, 8).toString();
        Assertions.assertEquals("f9", str);

        str = new Hexagon(6, 9).toString();
        Assertions.assertEquals("g10", str);

        str = new Hexagon(4, 4).toString();
        Assertions.assertEquals("e5", str);

        str = new Hexagon(6, 0).toString();
        Assertions.assertEquals("g1", str);
    }

    @Test
    public void testHexagonFromString() {
        Hexagon hex;

        hex = Hexagon.fromNotation("f9");
        Assertions.assertEquals(new Hexagon(5, 8), hex);

        hex = Hexagon.fromNotation("g10");
        Assertions.assertEquals(new Hexagon(6, 9), hex);

        hex = Hexagon.fromNotation("e5");
        Assertions.assertEquals(new Hexagon(4, 4), hex);
    }

    @Test
    public void testGetSetPieces() {
        ChessBoard board = ChessBoard.initial();

        board.setPiece("f3", WHITE_BISHOP);
        byte piece = board.getPiece("f3");

        Assertions.assertEquals(WHITE_BISHOP, piece);
    }

    private void assertPieceMoves(List<Hexagon> actualMoves, String... expMoves) {
        List<Hexagon> expectedMoves = Stream.of(expMoves).map(Hexagon::fromNotation).toList();
        Assertions.assertTrue(CollectionUtils.isEqualCollection(expectedMoves, actualMoves));
    }

    @Test
    public void testFindRookMoves() {
        // given
        ChessGame game1 = ChessGame.start().setPiece("f6", BLACK_ROOK);
        ChessGame game2 = ChessGame.start().setPiece("c8", BLACK_ROOK);
        ChessGame game3 = ChessGame.start().setPiece("h4", BLACK_ROOK);
        // when
        List<Hexagon> centerMoves = game1.findRookMoves(Hexagon.fromNotation("f6")).getMoves();
        List<Hexagon> leftMoves = game2.findRookMoves(Hexagon.fromNotation("c8")).getMoves();
        List<Hexagon> rightMoves = game3.findRookMoves(Hexagon.fromNotation("h4")).getMoves();

        LOG.info(game1.getBoard().toMovesString(centerMoves));
        LOG.info(game2.getBoard().toMovesString(leftMoves));
        LOG.info(game3.getBoard().toMovesString(rightMoves));

        // then
        assertPieceMoves(centerMoves,
            "f5", "e5", "d4", "c3", "b2", "a1", "g5", "h4", "i3", "j2", "k1",
            "e6", "d6", "c6", "b6", "a6", "g6", "h6", "i6", "j6", "k6");

        assertPieceMoves(leftMoves, "d8", "e8", "f8");

        assertPieceMoves(rightMoves,
            "h5", "h6", "h3", "g4", "i3", "j2", "k1", "g5", "f6", "e6",
            "d6", "c6", "b6", "a6", "i4", "j4", "k4");
    }

    @Test
    public void testFindBishopMoves() {
        // given
        ChessGame game1 = ChessGame.start().setPiece("f6", BLACK_BISHOP);
        ChessGame game2 = ChessGame.start().setPiece("c8", BLACK_BISHOP);
        ChessGame game3 = ChessGame.start().setPiece("h4", BLACK_BISHOP);

        // when
        List<Hexagon> centerMoves = game1.findBishopMoves(Hexagon.fromNotation("f6")).getMoves();
        List<Hexagon> leftMoves = game2.findBishopMoves(Hexagon.fromNotation("c8")).getMoves();
        List<Hexagon> rightMoves = game3.findBishopMoves(Hexagon.fromNotation("h4")).getMoves();

        LOG.info(game1.getBoard().toMovesString(centerMoves));
        LOG.info(game2.getBoard().toMovesString(leftMoves));
        LOG.info(game3.getBoard().toMovesString(rightMoves));

        // then
        assertPieceMoves(centerMoves,
            "h5", "j4", "d5", "b4", "g4", "e4");

        assertPieceMoves(leftMoves,
            "e9", "g9", "b6", "a4");

        assertPieceMoves(rightMoves,
            "j3", "f5", "i5", "j6", "g6", "f8", "e9", "i2", "g3", "f2");
    }

    @Test
    public void testFindKingMoves() {
        // given
        ChessGame game1 = ChessGame.empty().setPiece("f6", WHITE_KING);
        ChessGame game2 = ChessGame.empty().setPiece("d3", WHITE_KING);
        ChessGame game3 = ChessGame.empty().setPiece("h7", WHITE_KING);

        // when
        List<Hexagon> centerMoves = game1.findKingMoves(Hexagon.fromNotation("f6"));
        List<Hexagon> leftMoves = game2.findKingMoves(Hexagon.fromNotation("d3"));
        List<Hexagon> rightMoves = game2.findKingMoves(Hexagon.fromNotation("h7"));

        LOG.info(game3.getBoard().toMovesString(centerMoves));
        LOG.info(game3.getBoard().toMovesString(leftMoves));
        LOG.info(game3.getBoard().toMovesString(rightMoves));

        // then
        assertPieceMoves(centerMoves,
            "f7", "f5", "e5", "g5", "e6", "g6", "h5", "d5", "g7", "e7", "g4", "e4");

        assertPieceMoves(leftMoves,
            "d4", "d2", "c2", "e3", "c3", "e4", "f4", "b2", "e5", "c4", "e2", "c1");

        assertPieceMoves(rightMoves,
            "h8", "h6", "g7", "i6", "g8", "i7", "j6", "f8", "i8", "g9", "i5", "g6");
    }

    @Test
    public void testFindKnightMoves() {
        // given
        ChessGame game1 = ChessGame.empty().setPiece("f6", WHITE_KNIGHT);
        ChessGame game2 = ChessGame.empty().setPiece("d3", WHITE_KNIGHT);
        ChessGame game3 = ChessGame.empty().setPiece("h7", WHITE_KNIGHT);

        // when
        List<Hexagon> centerMoves = game1.findKnightMoves(Hexagon.fromNotation("f6")).getMoves();
        List<Hexagon> leftMoves = game2.findKnightMoves(Hexagon.fromNotation("d3")).getMoves();
        List<Hexagon> rightMoves = game3.findKnightMoves(Hexagon.fromNotation("h7")).getMoves();

        LOG.info(game1.getBoard().toMovesString(centerMoves));
        LOG.info(game2.getBoard().toMovesString(leftMoves));
        LOG.info(game3.getBoard().toMovesString(rightMoves));

        // then
        assertPieceMoves(centerMoves,
            "h7", "g8", "h3", "g3", "d7", "e8", "d3", "e3", "c5", "c4", "i5", "i4");

        assertPieceMoves(leftMoves,
            "f6", "e6", "f2", "e1", "b4", "c5", "a2", "a1", "g4", "g3");

        assertPieceMoves(rightMoves,
            "j4", "i4", "f10", "g10", "f6", "g5", "e8", "e7", "k6", "k5");
    }

    @Test
    public void testFindPawnMoves() {
        // given
        ChessGame game1 = ChessGame.empty().setPiece("g4", WHITE_PAWN);
        ChessGame game2 = ChessGame.empty()
            .setPiece("c4", BLACK_KNIGHT)
            .setPiece("e5", WHITE_KNIGHT)
            .setPiece("d5", BLACK_PAWN);

        // when
        List<Hexagon> firstMoves = game1.findPawnMoves(Hexagon.fromNotation("g4"), true).getMoves();
        List<Hexagon> takeMoves = game2.findPawnMoves(Hexagon.fromNotation("d5"), false).getMoves();

        LOG.info(game1.getBoard().toMovesString(firstMoves));
        LOG.info(game2.getBoard().toMovesString(takeMoves));

        // then
        assertPieceMoves(firstMoves, "g5", "g6");

        assertPieceMoves(takeMoves, "d4", "e5");
    }

    @Test
    public void testDetermineIsCheckmate() {
        ChessGame game = ChessGame.empty()
            .setPiece("f6", WHITE_KING)
            .setPiece("f4", BLACK_QUEEN)
            .setPiece("f8", BLACK_QUEEN)
            .setPiece("b4", BLACK_BISHOP)
            .setPiece("j4", BLACK_BISHOP)
            .setPiece("f9", BLACK_KING);

        game.initPieceMoves();

        LOG.info(game.getBoard().toString());
        LOG.info(game.getBoard().toPieceMovesString(game.findOppositeMoves()));

        boolean isCheckmate = game.checkmateReached();
        Assertions.assertTrue(isCheckmate);
    }

//    @Test
//    public void testInitPieceMoves() {
//        // given
//        ChessGame game = ChessGame.start();
//
//        // when
//        game.initPieceMoves();
//        List<PieceMoves> currMoves = game.findCurrMoves();
//        List<PieceMoves> oppMoves = game.findOppositeMoves();
//
//        // then
//        List<PieceMoves> expectedCurrMoves = new ArrayList<>();
//        List<PieceMoves> expectedOppMoves = new ArrayList<>();
//        Assertions.assertEquals(expectedCurrMoves, currMoves);
//        Assertions.assertEquals(expectedOppMoves, oppMoves);
//    }
}
