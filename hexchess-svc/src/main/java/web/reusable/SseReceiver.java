package web.reusable;

import io.jooby.ServerSentEmitter;
import services.broadcast.Receiver;

public class SseReceiver<Content> extends Receiver<Content> {
    private final ServerSentEmitter sse;
    private final String event;

    public SseReceiver(String id, ServerSentEmitter sse, String event) {
        super(id);
        this.sse = sse;
        this.event = event;
    }

    @Override
    public void onMessage(Content content) {
        sse.send(event, content);
    }

    @Override
    public void onEviction() {
        if (sse.isOpen()) {
            sse.close();
        }
    }

    @Override
    public boolean isClosed() {
        return !sse.isOpen();
    }
}