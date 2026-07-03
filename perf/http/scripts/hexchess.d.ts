declare module "k6/x/hexchess/websocket" {
    export type OutputMetric = {
        outputTime: number;
        outputType: string;
    }

    export type Metric = {
        inputTime: number;
        inputType: string;
        outputs: OutputMetric[]
    }

    export function runGameSockets(
        req: {
            targetEndpoint: string;
            gameId: string;
            sessionIds: string[];
            staggerMs: number;
            timeoutSecs: number;
            maxMoves: number;
        }
    ): {
        metrics: Record<string, Metric>;
        error?: string;
    };
}