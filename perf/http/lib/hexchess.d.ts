declare module "k6/x/hexchess/websocket" {
    export function handleGameInputEvent(req: {
        recvBytes: ArrayBuffer,
        inputMessageId: string
    }): {
        prevMessageId?: string;
        prevType?: string;
        isTerminal?: boolean;
        nextInputBytes?: ArrayBuffer;
        error?: string;
    };
}