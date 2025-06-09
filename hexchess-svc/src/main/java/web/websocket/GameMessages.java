package web.websocket;

import chess.ChessGame;
import chess.PieceMove;
import messages.Messages;
import models.state.ChessRoom;
import models.state.Player;

public class GameMessages {
    public static byte[] serializeError(String message) {
        return Messages.GameOutput.newBuilder()
            .setError(Messages.Error.newBuilder().setMessage(message))
            .build()
            .toByteArray();
    }

    public static byte[] serializeStart(Player self, ChessRoom room) {
        Messages.Init.Builder init = Messages.Init.newBuilder()
            .setRoom(room.serialize());
        if (self != null) {
            init.setSelf(self.serialize());
        }
        return Messages.GameOutput.newBuilder()
            .setInit(init.build())
            .build()
            .toByteArray();
    }

    public static byte[] serializePlayers(Player whitePlayer, Player blackPlayer) {
        Messages.Players.Builder players = Messages.Players.newBuilder();
        if (whitePlayer != null) {
            players.setWhitePlayer(whitePlayer.serialize());
        }
        if (blackPlayer != null) {
            players.setBlackPlayer(blackPlayer.serialize());
        }
        return Messages.GameOutput.newBuilder()
            .setPlayers(players.build())
            .build()
            .toByteArray();
    }

    public static byte[] serializeMove(PieceMove pieceMove, ChessGame game) {
        Messages.Move move = Messages.Move.newBuilder()
            .setPieceMove(pieceMove.serialize())
            .setGame(game.serialize())
            .build();
        return Messages.GameOutput.newBuilder()
            .setMove(move)
            .build()
            .toByteArray();
    }

    public static byte[] serializeChat(Player player, String message) {
        Messages.Chat chat = Messages.Chat.newBuilder()
            .setPlayer(player.serialize())
            .setMessage(message)
            .build();
        return Messages.GameOutput.newBuilder()
            .setChat(chat)
            .build()
            .toByteArray();
    }

    public static byte[] serializeForfeit() {
        return Messages.GameOutput.newBuilder()
            .setForfeit(Messages.Forfeit.newBuilder())
            .build()
            .toByteArray();
    }
}
