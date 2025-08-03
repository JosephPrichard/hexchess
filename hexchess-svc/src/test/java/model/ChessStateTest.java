package model;

import com.google.protobuf.InvalidProtocolBufferException;
import messages.Messages;
import models.enums.TimeControl;
import models.state.ChessState;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.Test;

import java.util.UUID;

import static utils.Globals.LOG;

public class ChessStateTest {
    @Test
    public void testRoomSerializerNotStarted() throws InvalidProtocolBufferException {
        // given
        ChessState in = ChessState.startWithGame(UUID.randomUUID().toString(), TimeControl.REAL_TIME);

        // when
        byte[] bytes = in.serializeAsBytes();
        Messages.ChessState msg = Messages.ChessState.parseFrom(bytes);
        ChessState out = ChessState.deserialize(msg);

        LOG.info("Deserialized room={}, board={}", out, out.getGame().getBoard());

        // then
        Assertions.assertEquals(in, out);
    }

    @Test
    public void testRoomSerializer() throws InvalidProtocolBufferException {
        // given
        ChessState in = ChessState.startWithGame(UUID.randomUUID().toString(), TimeControl.REAL_TIME);
        in.getGame().initPieceMoves();

        // when
        byte[] bytes = in.serializeAsBytes();
        Messages.ChessState msg = Messages.ChessState.parseFrom(bytes);
        ChessState out = ChessState.deserialize(msg);

        LOG.info("Deserialized room={}, board={}", out, out.getGame().getBoard());

        // then
        Assertions.assertEquals(in, out);
    }
}
