package services.broadcast;

import lombok.AllArgsConstructor;
import lombok.Data;

import java.util.function.Consumer;

@Data
@AllArgsConstructor
public class Handler<Content> {
    private String handlerId;
    private Consumer<Content> consumer;

    @Override
    public String toString() {
        return handlerId;
    }
}

