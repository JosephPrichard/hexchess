package chess;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.util.List;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class PieceMoves {
    public Hexagon hex;
    public List<Hexagon> moves;
}