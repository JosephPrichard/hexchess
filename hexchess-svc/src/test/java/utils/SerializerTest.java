package utils;

import com.esotericsoftware.kryo.Kryo;
import com.esotericsoftware.kryo.io.Input;
import com.esotericsoftware.kryo.io.Output;
import models.state.ChessRoom;
import models.common.TimeControl;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.Test;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;

public class SerializerTest {

    @Test
    public void testRoundTrip() {
        // given
        ChessRoom match = ChessRoom.startWithGame("1", TimeControl.UNLIMITED);

        // when
        ByteArrayOutputStream rawBytesOut = new ByteArrayOutputStream();
        try (Output output = new Output(rawBytesOut)) {
            Kryo kryo = Serializer.get();
            kryo.writeObject(output, match);
        }

        ChessRoom afterChessRoom;
        ByteArrayInputStream rawBytesIn = new ByteArrayInputStream(rawBytesOut.toByteArray());
        try (Input input = new Input(rawBytesIn)) {
            Kryo kryo = Serializer.get();
            afterChessRoom = kryo.readObject(input, ChessRoom.class);
        }

        // then
        Assertions.assertEquals(match, afterChessRoom);
    }
}
