package chess;

import lombok.*;

@Data
@AllArgsConstructor
public class Move {
    private Hexagon from;
    private Hexagon to;
}