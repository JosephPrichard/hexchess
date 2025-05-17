package web.dto;

import chess.ChessGame;
import models.entities.PlayerEntity;
import models.state.GameState;

public sealed interface GameOutput permits
        GameOutput.Error,
        GameOutput.Forfeit,
        GameOutput.Connect,
        GameOutput.Join,
        GameOutput.Move,
        GameOutput.Chat {

    String getType();

    record Error(String message) implements GameOutput {
        public String getType() {
            return "ERROR";
        }
    }

    record Forfeit(GameState gameState) implements GameOutput {
        public String getType() {
            return "FORFEIT";
        }
    }

    record Connect(PlayerEntity player) implements GameOutput {
        public String getType() {
            return "CONNECT";
        }
    }

    record Join(GameState gameState) implements GameOutput {
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
}