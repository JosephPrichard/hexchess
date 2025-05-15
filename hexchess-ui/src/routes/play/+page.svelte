<script lang="ts">
    import type { GameOutputMsg } from '$lib/models';
    import Banner from '$lib/components/Banner.svelte';
    import { postTempSession, unwrap } from '$lib/api';
    import { createMessage } from '$lib/error';
    import { getNotificationsContext } from '$lib/context';

    interface Props {
        data: {
            gameId: string;
        }
    }

    const { data: props }: Props = $props();

    const { addNotification } = getNotificationsContext();

    let ws: WebSocket | undefined = undefined;
    let connectTries = 0;

    function onMessage(message: GameOutputMsg) {
        console.log("Received message", message);
        switch (message.type) {
        case 'ERROR':
            // handleErrorMessage(data);
            break;
        case 'FORFEIT':
            // handleForfeitMessage(data);
            break;
        case 'CONNECT':
            // handleConnectMessage(data);
            break;
        case 'JOIN':
            // handleJoinMessage(data);
            break;
        case 'MOVE':
            // handleMoveMessage(data);
            break;
        case 'CHAT':
            // handleTextMessage(data);
            break;
        default:
            console.error('Unknown error message type: ' + message.type);
        }
    }

    function connectGame(gameId: string) {
        setTimeout(async () => {
            let { ok, resp: sessionId, err } = await unwrap(postTempSession());
            if (ok && sessionId) {
                const params = new URLSearchParams({ sessionId });
                let url =  `/connections/games/${gameId}?${params}`;

                ws = new WebSocket(url);
                ws.addEventListener('open', () => {
                    connectTries = 0;
                });
                ws.addEventListener('message', (event) => {
                    const data: GameOutputMsg = JSON.parse(event.data);
                    onMessage(data);
                });
                ws.addEventListener('error', () => {
                    console.log(`Disconnected from game=${gameId} with error, trying to reconnect with ${connectTries} tries`);
                    connectTries += 1;
                    connectGame(gameId);
                });
            } else {
                const message = createMessage(err);
                addNotification({ type: 'string', message, isSuccess: false }, 3000);
            }
        }, connectTries !== 0 ? Math.pow(2, connectTries) * 1000 : 0);
    }

    $effect(() => {
        connectGame(props.gameId);
        return () => {
            if (ws) {
                ws.close();
            }
        };
    });
</script>

<Banner />