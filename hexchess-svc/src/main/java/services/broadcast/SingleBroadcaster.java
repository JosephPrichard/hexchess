package services.broadcast;

import java.util.function.Consumer;

public interface SingleBroadcaster {
    String USERS_COUNT_TOPIC = "USERS_COUNT";
    String GAME_COUNT_TOPIC = "GAMES_COUNT";

    void subscribe(String handlerId, Consumer<String> consumer);

    void unsubscribe(String handlerId);

    void broadcast(String content);
}
