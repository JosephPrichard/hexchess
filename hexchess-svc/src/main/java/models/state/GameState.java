package models.state;

import com.fasterxml.jackson.annotation.JsonIgnore;
import chess.*;
import com.fasterxml.jackson.annotation.JsonProperty;
import lombok.*;
import models.common.ColorSelect;
import models.common.TimeControl;
import models.entities.PlayerEntity;

import java.util.ArrayList;
import java.util.List;

@ToString
@EqualsAndHashCode
@NoArgsConstructor
@AllArgsConstructor
public class GameState {
    public String id;
    @ToString.Exclude
    public ChessGame game;
    public PlayerEntity whitePlayer;
    public PlayerEntity blackPlayer;
    public boolean isEnded;
    @JsonIgnore
    public ColorSelect firstColor; // decides what color the first joining selfPlayer joins as
    public TimeControl timeControl;
    @EqualsAndHashCode.Exclude
    @JsonIgnore
    public long touch;
    public List<PieceMove> moveList;

    public static GameState startWithGame(String id, TimeControl timeControl) {
        ChessGame game = ChessGame.start();
        List<PieceMove> moveList = new ArrayList<>();
        return new GameState(id, game, null, null, false, ColorSelect.RANDOM, timeControl, 0, moveList);
    }

    public static GameState ofPlayers(String id, PlayerEntity whitePlayer, PlayerEntity blackPlayer) {
        return new GameState(id, null, whitePlayer, blackPlayer, false, ColorSelect.RANDOM, null, 0, null);
    }

    @JsonIgnore
    public PlayerEntity getCurrPlayer() {
        return game.getBoard().turn().isWhite() ? whitePlayer : blackPlayer;
    }

    public boolean isStarted() {
        return whitePlayer != null && blackPlayer != null;
    }
}
