import { Counter, Trend } from "k6/metrics";

export class TrendCounter {
    trend: Trend;
    count: Counter;

    constructor(name: string) {
        this.trend = new Trend(name, true);
        this.count = new Counter(`${name}_count`);
    }

    add(value: number | boolean, tags?: {[name: string]: string}) {
        this.trend.add(value, tags);
        this.count.add(1, tags);
    }
}
    