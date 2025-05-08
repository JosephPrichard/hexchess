package services.broadcast;

import java.util.function.Consumer;

public interface Broadcaster {
    String GAMES_TOPIC = "GAMES";
    String USERS_TOPIC = "NOTIFICATIONS";

    void subscribe(String groupId, String handlerId, Consumer<String> consumer);

    void unsubscribe(String groupId, String handlerId);

    void broadcast(String groupId, String content);
}
