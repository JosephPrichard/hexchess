package services;

import java.util.function.Consumer;

public interface Broadcaster {
    String GAMES_CHANNEL = "GAMES";
    String USERS_CHANNEL = "NOTIFICATIONS";

    void subscribe(String groupId, String handlerId, Consumer<String> consumer);

    void unsubscribe(String groupId, String handlerId);

    void broadcast(String groupId, String content);
}
