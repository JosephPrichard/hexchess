package web.dto;

import chess.Move;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class GameInputMsg {
    public static final int FORFEIT = 0;
    public static final int MOVE = 1;
    public static final int TEXT = 2;

    public int type;
    public Move move;
    public String message;
}
