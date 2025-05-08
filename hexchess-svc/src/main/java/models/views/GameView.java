package models.views;

import lombok.*;
import models.state.GameState;

import static utils.Globals.JSON_MAPPER;

@ToString
@EqualsAndHashCode
@Getter
public class GameView {
    public String id;
    public Long whiteId;
    public Long blackId;
    public String whiteName;
    public String blackName;
    public String whiteCountry;
    public String blackCountry;
    public int blackScore;
    public int whiteScore;
    public String boardJson;

    @SneakyThrows
    public static GameView create(GameState gameState) {
        GameView view = new GameView();
        view.id = gameState.id;
        view.boardJson = JSON_MAPPER.writeValueAsString(gameState.game.getBoard());
        if (gameState.whitePlayer != null) {
            view.whiteId = gameState.whitePlayer.id;
            view.whiteName = gameState.whitePlayer.name;
            view.whiteCountry = gameState.whitePlayer.country;
        }
        if (gameState.blackPlayer != null) {
            view.blackId = gameState.blackPlayer.id;
            view.blackName = gameState.blackPlayer.name;
            view.blackCountry = gameState.blackPlayer.country;
        }
        return view;
    }
}
