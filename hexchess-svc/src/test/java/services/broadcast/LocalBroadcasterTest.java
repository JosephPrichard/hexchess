package services.broadcast;

import org.junit.jupiter.api.Test;

import java.util.function.Consumer;

import static org.mockito.Mockito.*;

public class LocalBroadcasterTest {

    @Test
    @SuppressWarnings("unchecked")
    public void testLocalBroadcaster() {
        // given
        LocalBroadcaster broadcaster = new LocalBroadcaster("test-broadcaster");

        Consumer<byte[]> handler1 = mock(Consumer.class);
        Consumer<byte[]> handler2 = mock(Consumer.class);
        Consumer<byte[]> handler3 = mock(Consumer.class);
        Consumer<byte[]> handler4 = mock(Consumer.class);

        // when
        broadcaster.subscribe("groupId1", "handlerId1", handler1);
        broadcaster.subscribe("groupId1", "handlerId2", handler2);
        broadcaster.subscribe("groupId1", "handlerId3", handler3);
        broadcaster.subscribe("groupId2", "handlerId4", handler4);

        broadcaster.broadcast("groupId1", "Test1".getBytes());
        broadcaster.unsubscribe("groupId1", "handlerId1");
        broadcaster.broadcast("groupId1", "Test2".getBytes());
        broadcaster.broadcast("group", "Test3".getBytes());

        // then
        verify(handler1).accept("Test1".getBytes());
        verify(handler2).accept("Test1".getBytes());
        verify(handler3).accept("Test1".getBytes());

        verify(handler1, times(0)).accept("Test2".getBytes());
        verify(handler2).accept("Test2".getBytes());
        verify(handler3).accept("Test2".getBytes());

        verify(handler1, times(0)).accept("Test3".getBytes());
        verify(handler2, times(0)).accept("Test3".getBytes());
        verify(handler3, times(0)).accept("Test3".getBytes());

        verify(handler4, times(0)).accept(any());
    }
}
