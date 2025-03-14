package domain;

import com.fasterxml.jackson.core.JsonGenerator;
import com.fasterxml.jackson.databind.SerializerProvider;
import com.fasterxml.jackson.databind.ser.std.StdSerializer;

import java.io.IOException;

public class BoardSerializer extends StdSerializer<ChessBoard> {

    public BoardSerializer() {
        this(null);
    }

    public BoardSerializer(Class<ChessBoard> t) {
        super(t);
    }

    @Override
    public void serialize(ChessBoard board, JsonGenerator json, SerializerProvider provider) throws IOException {
        json.writeStartObject();

        json.writeFieldName("turn");
        json.writeNumber(board.turn().toInt());

        json.writeFieldName("pieces");
        json.writeStartArray();

        byte[][] pieces = board.getPieces();
        for (byte[] piecesFile : pieces) {
            json.writeStartArray();
            for (byte b : piecesFile) {
                json.writeNumber(b);
            }
            json.writeEndArray();
        }

        json.writeEndArray();

        json.writeEndObject();
    }
}