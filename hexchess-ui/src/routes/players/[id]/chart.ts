import {type EloBuckets, GameModeNameMap} from "$lib/api/models";
import {generateColors} from "$lib/utils/colors";
import {Chart} from "chart.js";
import {formatTimestamp, normalizeToDay} from "$lib/utils/format";

const chartsJsFont = "-apple-system, system-ui, BlinkMacSystemFont, \'Segoe UI\', Roboto, Ubuntu";

export function makeEloHistoriesChart(ctx: CanvasRenderingContext2D, buckets: Record<string, EloBuckets>) {
    const entries = Object.entries(buckets);
    const colors = generateColors(entries.length);

    let maxElo = Math.max(...entries.flatMap(([, eloHistories]) => eloHistories.map((h) => h.elo)));
    if (maxElo === 0) {
        maxElo = 1000;
    } else {
        maxElo *= 2;
    }

    return new Chart(ctx, {
        type: "line",
        data: {
            datasets: entries.map(([mode, eloHistories], i) => ({
                label: GameModeNameMap[mode] || "Unknown",
                data: eloHistories.map((h) => ({ x: normalizeToDay(h.timestamp), y: h.elo})),
                borderWidth: 2,
                tension: 0.25,
                pointRadius: 3,
                borderColor: colors[i](1),
                backgroundColor: colors[i](0.2),
                fill: true,
                pointBackgroundColor: colors[i](1)
            }))
        },
        options: {
            responsive: true,
            plugins: {
                legend: {
                    labels: {
                        font: {
                            family: chartsJsFont,
                            size: 14,
                            weight: 'bold',
                        }
                    }
                }
            },
            scales: {
                x: {
                    type: "time",
                    grid: {
                        color: 'rgb(55,55,55)'
                    },
                    time: {
                        minUnit: 'day',
                    },
                    ticks: {
                        callback: formatTimestamp,
                        font: {
                            family: chartsJsFont
                        }
                    },
                    title: {
                        display: true,
                        text: 'Date',
                        font: {
                            size: 14,
                            weight: 'bold'
                        },
                        color: 'rgb(120,120,120)'
                    }
                },
                y: {
                    suggestedMin: 0,
                    suggestedMax: maxElo,
                    beginAtZero: false,
                    grid: {
                        color: 'rgb(55,55,55)'
                    },
                    ticks: {
                        callback: (value) => String(value),
                        font: {
                            family: chartsJsFont
                        }
                    },
                    title: {
                        display: true,
                        text: 'Elo',
                        font: {
                            size: 14,
                            family: chartsJsFont,
                            weight: 'bold'
                        },
                        color: 'rgb(120,120,120)'
                    }
                }
            }
        },
    });
}