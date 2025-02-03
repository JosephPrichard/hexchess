package domain;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class Move {
    public Hexagon from;
    public Hexagon to;

    public Move deepCopy() {
        return new Move(from.deepCopy(), to.deepCopy());
    }
}