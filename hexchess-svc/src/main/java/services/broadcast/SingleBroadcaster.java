package services.broadcast;

public interface SingleBroadcaster {
    String USERS_COUNT_TOPIC = "USERS_COUNT";
    String GAME_COUNT_TOPIC = "GAMES_COUNT";

    void subscribe(Receiver<String> receiver);

    void unsubscribe(String handlerId);

    void broadcast(String content);
}
