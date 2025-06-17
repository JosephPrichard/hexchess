package services.broadcast;

public interface Broadcaster {
    String GAMES_TOPIC = "GAMES";
    String USERS_TOPIC = "NOTIFICATIONS";

    void subscribe(String groupId, Receiver<byte[]> receiver);

    void unsubscribe(String groupId, String handlerId);

    void broadcast(String groupId, byte[] content);
}
