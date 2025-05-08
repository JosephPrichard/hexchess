<script lang="ts">
    import type { WsMessage } from '$lib/models';
    import { onMount } from 'svelte';
    import { getClientSession } from '$lib/utils';
    import Banner from '$lib/components/Banner.svelte';

    interface Props {
        data: {
            gameId: string;
        }
    }

    const { data }: Props = $props();
    const { gameId } = data;

    let ws = undefined;
    let connectTries = 0;

    function onMessage(message: WsMessage) {
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

    function connectGame(sessionId: string | undefined) {
        setTimeout(function () {
            let url =  `/connections/games/${gameId}`;
            if (sessionId !== undefined) {
                url += `?sessionId=${sessionId}`;
            }

            ws = new WebSocket(url);
            ws.addEventListener('open', () => {
                connectTries = 0;
            });
            ws.addEventListener('message', (event) => {
                const data: WsMessage = JSON.parse(event.data);
                onMessage(data);
            });
            ws.addEventListener('error', () => {
                console.log(`Disconnected with error, trying to reconnect with ${connectTries} tries`);
                connectTries += 1;
                connectGame(sessionId);
            });
        }, connectTries !== 0 ? Math.pow(2, connectTries) * 1000 : 0);
    }

    onMount(() => {
        let session = getClientSession();
        connectGame(session?.sessionId);
    });
</script>

<Banner />