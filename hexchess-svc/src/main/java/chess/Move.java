package chess;

import lombok.*;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class Move {
    private Hexagon from;
    private Hexagon to;
}