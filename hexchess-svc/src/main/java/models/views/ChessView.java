package models.views;

import com.google.protobuf.InvalidProtocolBufferException;
import lombok.AllArgsConstructor;
import lombok.Data;
import messages.Messages;
import models.enums.ColorSelect;
import models.enums.TimeControl;
import models.state.PlayerState;

import javax.annotation.Nullable;

import static utils.Globals.LOG;

@Data
@AllArgsConstructor
public class ChessView {
    private String id;
    @Nullable
    private PlayerState whitePlayer;
    @Nullable
    private PlayerState blackPlayer;
    private boolean isEnded;
    private ColorSelect firstColor;
    private TimeControl timeControl;

    public static ChessView deserialize(Messages.ChessState msg) {
        String id = msg.getId();

        PlayerState blackPlayer = msg.hasBlackPlayer() ? PlayerState.deserialize(msg.getBlackPlayer()) : null;
        PlayerState whitePlayer = msg.hasWhitePlayer() ? PlayerState.deserialize(msg.getWhitePlayer()) : null;

        TimeControl timeControl = TimeControl.fromInt(msg.getTimeControl());
        ColorSelect firstColor = ColorSelect.fromInt(msg.getFirstColor());

        return new ChessView(id, whitePlayer, blackPlayer, msg.getIsEnded(), firstColor, timeControl);
    }

    public static ChessView deserialize(byte[] bytes) {
        try {
            return deserialize(Messages.ChessState.parseFrom(bytes));
        } catch (InvalidProtocolBufferException ex) {
            LOG.error("Failed to deserialize view object from bytes", ex);
            throw new RuntimeException(ex);
        }
    }
}
