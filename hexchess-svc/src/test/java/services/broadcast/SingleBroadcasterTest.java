package services.broadcast;

import org.junit.jupiter.api.Test;

import static org.mockito.Mockito.*;

public class SingleBroadcasterTest {
    @Test
    @SuppressWarnings("unchecked")
    public void testLocalBroadcaster() {
        // given
        SingleBroadcaster.Local broadcaster = new  SingleBroadcaster.Local("test-broadcaster");

        BroadcastReceiver<String> receiver1 = mock(BroadcastReceiver.class);
        BroadcastReceiver<String> receiver2 = mock(BroadcastReceiver.class);
        BroadcastReceiver<String> receiver3 = mock(BroadcastReceiver.class);

        // when
        when(receiver1.getId()).thenReturn("handlerId1");
        when(receiver2.getId()).thenReturn("handlerId2");
        when(receiver3.getId()).thenReturn("handlerId3");

        broadcaster.subscribe(receiver1);
        broadcaster.subscribe(receiver2);
        broadcaster.subscribe(receiver3);

        broadcaster.broadcast("Test1");
        broadcaster.unsubscribe( "handlerId1");
        broadcaster.broadcast("Test2");

        // then
        verify(receiver1).onMessage("Test1");
        verify(receiver2).onMessage("Test1");
        verify(receiver3).onMessage("Test1");

        verify(receiver1, times(0)).onMessage("Test2");
        verify(receiver2).onMessage("Test2");
        verify(receiver3).onMessage("Test2");
    }
}
