package models.state;

import chess.*;
import com.google.protobuf.InvalidProtocolBufferException;
import lombok.*;
import messages.Messages;
import models.enums.ColorSelect;
import models.enums.TimeControl;

import javax.annotation.Nullable;
import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;

import static utils.Globals.LOG;

@Data
@AllArgsConstructor
public class ChessRoom {
    @NonNull
    private String id;
    @ToString.Exclude
    @NonNull
    private ChessGame game;
    @NonNull
    private List<PieceMove> moveList;
    @Nullable
    private Player whitePlayer;
    @Nullable
    private Player blackPlayer;
    private boolean isEnded;
    @NonNull
    private ColorSelect firstColor; // decides what color the first joining selfPlayer joins as
    @NonNull
    private TimeControl timeControl;
    @EqualsAndHashCode.Exclude
    private long touch;

    public static ChessRoom startWithGame(String id, TimeControl timeControl) {
        return new ChessRoom(id, ChessGame.start(), new ArrayList<>(), null, null, false, ColorSelect.RANDOM, timeControl, 0);
    }

    public Player getCurrPlayer() {
        return game.getBoard().isWhiteTurn() ? whitePlayer : blackPlayer;
    }

    public void addMove(PieceMove pm) {
        moveList.add(pm);
    }

    public Messages.ChessRoom serialize() {
        Stream<Messages.PieceMove> moveListStream = moveList.stream().map(PieceMove::serialize);

        Messages.ChessRoom.Builder builder = Messages.ChessRoom.newBuilder()
            .setId(id)
            .setGame(game.serialize())
            .addAllMoveList(moveListStream::iterator)
            .setTimeControl(timeControl.toInt())
            .setFirstColor(firstColor.toInt())
            .setIsEnded(isEnded)
            .setTouch(touch);

        if (blackPlayer != null) {
            builder.setBlackPlayer(blackPlayer.serialize());
        }
        if (whitePlayer!= null) {
            builder.setWhitePlayer(whitePlayer.serialize());
        }

        return builder.build();
    }

    public byte[] serializeAsBytes() {
        return serialize().toByteArray();
    }

    public static ChessRoom deserialize(Messages.ChessRoom msg) {
        String id = msg.getId();
        ChessGame game = ChessGame.deserialize(msg.getGame());

        List<PieceMove> moveList = msg.getMoveListList()
            .stream()
            .map(PieceMove::deserialize)
            .toList();

        Player blackPlayer = msg.hasBlackPlayer() ? Player.deserialize(msg.getBlackPlayer()) : null;
        Player whitePlayer = msg.hasWhitePlayer() ? Player.deserialize(msg.getWhitePlayer()) : null;

        TimeControl timeControl = TimeControl.fromInt(msg.getTimeControl());
        ColorSelect firstColor = ColorSelect.fromInt(msg.getFirstColor());

        boolean isEnded = msg.getIsEnded();
        long touch = msg.getTouch();

        return new ChessRoom(id, game, moveList, whitePlayer, blackPlayer, isEnded, firstColor, timeControl, touch);
    }

    public static ChessRoom deserialize(byte[] bytes) {
        try {
            return deserialize(Messages.ChessRoom.parseFrom(bytes));
        } catch (InvalidProtocolBufferException ex) {
            LOG.error("Failed to deserialize room object from bytes", ex);
            throw new RuntimeException(ex);
        }
    }
}
