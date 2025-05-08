package web.dto;

import chess.Move;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;
import models.entities.PlayerEntity;
import models.state.GameState;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class GameOutputMsg {
    public String type;
    public String message; // for text content messages like errors or user chats
    public PlayerEntity player; // for player data when a player joins
    public Move move; // for move data when move is made
    public GameState gameState; // for sending the game state on initial render and on each mutation

    public static GameOutputMsg ofError(String message) {
        return new GameOutputMsg("ERROR", message, null, null, null);
    }

    public static GameOutputMsg ofForfeit(GameState gameState) {
        return new GameOutputMsg("FORFEIT", null, null, null, gameState);
    }

    public static GameOutputMsg ofConnect(GameState gameState) {
        return new GameOutputMsg("CONNECT", null, null, null, gameState);
    }

    public static GameOutputMsg ofJoin(PlayerEntity player) {
        return new GameOutputMsg("JOIN", null, player, null, null);
    }

    public static GameOutputMsg ofMove(Move move, GameState gameState) {
        return new GameOutputMsg("MOVE", null, null, move, gameState);
    }

    public static GameOutputMsg ofText(String message) {
        return new GameOutputMsg("CHAT", message, null, null, null);
    }
}
