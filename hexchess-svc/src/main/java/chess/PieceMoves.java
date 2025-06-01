package chess;

import lombok.*;
import messages.Messages;

import java.util.Collections;
import java.util.List;
import java.util.Optional;
import java.util.stream.Stream;

@Data
@AllArgsConstructor
public class PieceMoves {
    private Hexagon hex;
    private List<Hexagon> moves;

    public Messages.PieceMoves serialize() {
        Stream<Messages.Hexagon> movesStream = Optional.ofNullable(moves)
            .orElse(Collections.emptyList())
            .stream()
            .map(Hexagon::serialize);

        return Messages.PieceMoves.newBuilder()
            .setHex(hex.serialize())
            .addAllMoves(movesStream::iterator)
            .build();
    }

    public static PieceMoves deserialize(Messages.PieceMoves msg) {
        return new PieceMoves(
            Hexagon.deserialize(msg.getHex()),
            msg.getMovesList().stream().map(Hexagon::deserialize).toList());
    }
}