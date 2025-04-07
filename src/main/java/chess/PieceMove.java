package chess;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class PieceMove {
    public byte piece;
    public Hexagon from;
    public Hexagon to;
}
