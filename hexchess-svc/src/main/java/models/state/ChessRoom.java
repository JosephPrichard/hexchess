package models.state;

import chess.*;
import lombok.*;
import models.common.ColorSelect;
import models.common.TimeControl;

import java.util.ArrayList;
import java.util.List;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class ChessRoom {
    private String id;

    @ToString.Exclude
    private ChessGame game;
    private List<PieceMove> moveList;

    private Player whitePlayer;
    private Player blackPlayer;
    private boolean isEnded;

    private ColorSelect firstColor; // decides what color the first joining selfPlayer joins as
    private TimeControl timeControl;

    @EqualsAndHashCode.Exclude
    private long touch;

    public static ChessRoom startWithGame(String id, TimeControl timeControl) {
        return new ChessRoom(id, ChessGame.start(), new ArrayList<>(), null, null, false, ColorSelect.RANDOM, timeControl, 0);
    }

    public static ChessRoom ofPlayers(String id, Player whitePlayer, Player blackPlayer) {
        return new ChessRoom(id, null, null, whitePlayer, blackPlayer, false, ColorSelect.RANDOM, null, 0);
    }

    public Player findCurrPlayer() {
        return game.getBoard().turn().isWhite() ? whitePlayer : blackPlayer;
    }
}
