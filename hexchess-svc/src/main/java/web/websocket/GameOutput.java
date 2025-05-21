package web.websocket;

import chess.ChessGame;
import models.common.TimeControl;
import models.entities.PlayerEntity;
import models.state.GameState;

public sealed interface GameOutput permits
        GameOutput.Error,
        GameOutput.Forfeit,
        GameOutput.Start,
        GameOutput.Join,
        GameOutput.Move,
        GameOutput.Chat {

    String getType();

    record Forfeit(GameState gameState) implements GameOutput {
        public String getType() {
            return "FORFEIT";
        }
    }

    record Start(PlayerEntity selfPlayer, GameState gameState) implements GameOutput {
        public String getType() {
            return "START";
        }
    }

    record Join(PlayerEntity whitePlayer, PlayerEntity blackPlayer, PlayerEntity selfPlayer) implements GameOutput {
        public String getType() {
            return "JOIN";
        }
    }

    record Move(chess.Move move, ChessGame game) implements GameOutput {
        public String getType() {
            return "MOVE";
        }
    }

    record Chat(PlayerEntity player, String message) implements GameOutput {
        public String getType() {
            return "CHAT";
        }
    }

    record Error(String message) implements GameOutput {
        public String getType() {
            return "ERROR";
        }
    }
}