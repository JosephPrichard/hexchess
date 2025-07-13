package services.broadcast;

import lombok.Data;

@Data
public abstract class BroadcastReceiver<Content> {
    protected String id;

    public BroadcastReceiver(String id) {
        this.id = id;
    }

    public abstract void onMessage(Content message);

    public void onEviction() {};

    public boolean isClosed() {
        return false;
    }

    @Override
    public String toString() {
        return id;
    }
}

