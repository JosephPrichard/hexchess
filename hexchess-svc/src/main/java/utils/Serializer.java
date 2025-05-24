package utils;

import chess.*;
import com.esotericsoftware.kryo.Kryo;
import com.esotericsoftware.kryo.io.Input;
import com.esotericsoftware.kryo.io.Output;
import models.state.ChessRoom;
import models.entities.PlayerEntity;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;

import static utils.Globals.LOG;

public class Serializer {

    private static final ThreadLocal<Kryo> KRYO = ThreadLocal.withInitial(() -> {
        Kryo kryo = new Kryo();
        kryo.register(Hexagon.class);
        kryo.register(PieceMoves.class);
        kryo.register(ChessBoard.class);
        kryo.register(ChessGame.class);
        kryo.register(PlayerEntity.class);
        kryo.register(PieceMove.class);
        kryo.register(ChessRoom.class);
        return kryo;
    });

    public static Kryo get() {
        return KRYO.get();
    }

    public static <T> byte[] serialize(T obj) {
        try {
            ByteArrayOutputStream rawBytes = new ByteArrayOutputStream();
            try (Output output = new Output(rawBytes)) {
                Kryo kryo = Serializer.get();
                kryo.writeObject(output, obj);
            }
            return rawBytes.toByteArray();
        } catch (Exception ex) {
            LOG.error("Error occurred while serializing: {}", String.valueOf(ex));
            throw new RuntimeException(ex);
        }
    }

    public static <T> T deserialize(byte[] bytes, Class<T> clazz) {
        try {
            ByteArrayInputStream rawBytes = new ByteArrayInputStream(bytes);
            try (Input input = new Input(rawBytes)) {
                Kryo kryo = Serializer.get();
                return kryo.readObject(input, clazz);
            }
        } catch (Exception ex) {
            LOG.error("Error occurred while deserializing: {}", String.valueOf(ex));
            throw new RuntimeException(ex);
        }
    }
}
