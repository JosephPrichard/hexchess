package services.broadcast;

import org.junit.jupiter.api.Test;

import static org.mockito.Mockito.*;

public class LocalBroadcasterTest {

    @Test
    @SuppressWarnings("unchecked")
    public void testLocalBroadcaster() {
        // given
        LocalBroadcaster broadcaster = new LocalBroadcaster("test-broadcaster");

        Receiver<byte[]> receiver1 = mock(Receiver.class);
        Receiver<byte[]> receiver2 = mock(Receiver.class);
        Receiver<byte[]> receiver3 = mock(Receiver.class);
        Receiver<byte[]> receiver4 = mock(Receiver.class);

        // when
        when(receiver1.getId()).thenReturn("handlerId1");
        when(receiver2.getId()).thenReturn("handlerId2");
        when(receiver3.getId()).thenReturn("handlerId3");
        when(receiver4.getId()).thenReturn("handlerId4");

        broadcaster.subscribe("groupId1", receiver1);
        broadcaster.subscribe("groupId1", receiver2);
        broadcaster.subscribe("groupId1", receiver3);
        broadcaster.subscribe("groupId2", receiver4);

        broadcaster.broadcast("groupId1", "Test1".getBytes());
        broadcaster.unsubscribe("groupId1", "handlerId1");
        broadcaster.broadcast("groupId1", "Test2".getBytes());
        broadcaster.broadcast("group", "Test3".getBytes());

        // then
        verify(receiver1).onMessage("Test1".getBytes());
        verify(receiver2).onMessage("Test1".getBytes());
        verify(receiver3).onMessage("Test1".getBytes());

        verify(receiver1, times(0)).onMessage("Test2".getBytes());
        verify(receiver2).onMessage("Test2".getBytes());
        verify(receiver3).onMessage("Test2".getBytes());

        verify(receiver1, times(0)).onMessage("Test3".getBytes());
        verify(receiver2, times(0)).onMessage("Test3".getBytes());
        verify(receiver3, times(0)).onMessage("Test3".getBytes());

        verify(receiver4, times(0)).onMessage(any());
    }
}
