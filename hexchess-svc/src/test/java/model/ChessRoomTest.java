package model;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.google.protobuf.InvalidProtocolBufferException;
import messages.Messages;
import models.common.TimeControl;
import models.state.ChessRoom;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.Test;

import java.util.UUID;

import static utils.Globals.LOG;

public class ChessRoomTest {
    @Test
    public void testRoomSerializerNotStarted() throws InvalidProtocolBufferException {
        // given
        ChessRoom in = ChessRoom.startWithGame(UUID.randomUUID().toString(), TimeControl.REAL_TIME);

        // when
        byte[] bytes = in.serializeAsBytes();
        Messages.ChessRoom msg = Messages.ChessRoom.parseFrom(bytes);
        ChessRoom out = ChessRoom.deserialize(msg);

        LOG.info("Deserialized room={}, board={}", out, out.getGame().getBoard());

        // then
        Assertions.assertEquals(in, out);
    }

    @Test
    public void testRoomSerializer() throws InvalidProtocolBufferException {
        // given
        ChessRoom in = ChessRoom.startWithGame(UUID.randomUUID().toString(), TimeControl.REAL_TIME);
        in.getGame().initPieceMoves();

        // when
        byte[] bytes = in.serializeAsBytes();
        Messages.ChessRoom msg = Messages.ChessRoom.parseFrom(bytes);
        ChessRoom out = ChessRoom.deserialize(msg);

        LOG.info("Deserialized room={}, board={}", out, out.getGame().getBoard());

        // then
        Assertions.assertEquals(in, out);
    }
}
