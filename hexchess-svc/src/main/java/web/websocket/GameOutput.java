package web.websocket;

import chess.ChessGame;
import models.state.Player;
import models.state.ChessRoom;

public sealed interface GameOutput permits
        GameOutput.Error,
        GameOutput.Forfeit,
        GameOutput.Start,
        GameOutput.Join,
        GameOutput.Move,
        GameOutput.Chat {

    String getType();

    record Forfeit(ChessRoom room) implements GameOutput {
        public String getType() {
            return "FORFEIT";
        }
    }

    record Start(Player selfPlayer, ChessRoom room) implements GameOutput {
        public String getType() {
            return "START";
        }
    }

    record Join(Player whitePlayer, Player blackPlayer, Player selfPlayer) implements GameOutput {
        public String getType() {
            return "JOIN";
        }
    }

    record Move(chess.Move move, ChessGame game) implements GameOutput {
        public String getType() {
            return "MOVE";
        }
    }

    record Chat(Player player, String message) implements GameOutput {
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