package web.websocket;

import chess.Move;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class GameInput {
    public String type;
    public Move move;
    public String message;
}
