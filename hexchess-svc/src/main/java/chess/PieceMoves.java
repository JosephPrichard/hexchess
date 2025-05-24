package chess;

import lombok.*;

import java.util.List;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class PieceMoves {
    private Hexagon hex;
    private List<Hexagon> moves;
}