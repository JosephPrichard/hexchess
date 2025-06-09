package models.views;

import com.google.protobuf.InvalidProtocolBufferException;
import lombok.AllArgsConstructor;
import lombok.Data;
import messages.Messages;
import models.common.ColorSelect;
import models.common.TimeControl;
import models.state.Player;

import javax.annotation.Nullable;

import static utils.Globals.LOG;

@Data
@AllArgsConstructor
public class ChessView {
    private String id;
    @Nullable
    private Player whitePlayer;
    @Nullable
    private Player blackPlayer;
    private boolean isEnded;
    private ColorSelect firstColor;
    private TimeControl timeControl;

    public static ChessView deserialize(Messages.ChessRoom msg) {
        String id = msg.getId();

        Player blackPlayer = msg.hasBlackPlayer() ? Player.deserialize(msg.getBlackPlayer()) : null;
        Player whitePlayer = msg.hasWhitePlayer() ? Player.deserialize(msg.getWhitePlayer()) : null;

        TimeControl timeControl = TimeControl.fromInt(msg.getTimeControl());
        ColorSelect firstColor = ColorSelect.fromInt(msg.getFirstColor());

        return new ChessView(id, whitePlayer, blackPlayer, msg.getIsEnded(), firstColor, timeControl);
    }

    public static ChessView deserialize(byte[] bytes) {
        try {
            return deserialize(Messages.ChessRoom.parseFrom(bytes));
        } catch (InvalidProtocolBufferException ex) {
            LOG.error("Failed to deserialize view object from bytes", ex);
            throw new RuntimeException(ex);
        }
    }
}
