package web.websocket;

import chess.Move;
import lombok.*;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class GameInput {
    private String type;
    private Move move;
    private String message;
}
