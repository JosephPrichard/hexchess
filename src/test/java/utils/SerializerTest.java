package utils;

import com.esotericsoftware.kryo.Kryo;
import com.esotericsoftware.kryo.io.Input;
import com.esotericsoftware.kryo.io.Output;
import models.GameState;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.Test;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;

public class SerializerTest {

    @Test
    public void testRoundTrip() {
        // given
        GameState match = GameState.startWithGame("1");

        // when
        ByteArrayOutputStream rawBytesOut = new ByteArrayOutputStream();
        try (Output output = new Output(rawBytesOut)) {
            Kryo kryo = Serializer.get();
            kryo.writeObject(output, match);
        }

        GameState afterGameState;
        ByteArrayInputStream rawBytesIn = new ByteArrayInputStream(rawBytesOut.toByteArray());
        try (Input input = new Input(rawBytesIn)) {
            Kryo kryo = Serializer.get();
            afterGameState = kryo.readObject(input, GameState.class);
        }

        // then
        Assertions.assertEquals(match, afterGameState);
    }
}
