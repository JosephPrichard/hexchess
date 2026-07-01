export type Metric = {
    inputTime: Date;
    inputType: string;
    outputs: { outputTime: Date; outputType: string }[];
}

export type OutputMetric = {
    outputTime: Date;
    outputType: string;
}

export class MetricMap {
    private metricMap: Map<string, Metric>;

    constructor() {
        this.metricMap = new Map();
    }

    public newMetric(messageId: string, inputType: string) {
        this.metricMap.set(messageId, {inputTime: new Date(), inputType: inputType, outputs: []});
    }

    appendMetric(messageId: string, outputType: string) {
        const metric = this.metricMap.get(messageId);
        if (!metric) {
            throw new Error(`unknown metric for message id ${messageId}`);
        }
        metric.outputs.push({outputTime: new Date(), outputType});
    }
    
    iterateMetrics(f: (metric: Metric, outputMetric: OutputMetric) => void) {
        for (const [_, metric] of this.metricMap) {
            for (const outputMetric of metric.outputs) {
                f(metric, outputMetric);
            }
        }
    }
}