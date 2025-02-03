package services;

import io.jooby.WebSocket;

public interface Broadcaster {
    void subscribe(String id, WebSocket ws);

    void unsubscribe(String id, WebSocket ws);

    void broadcast(String id, String content);
}
