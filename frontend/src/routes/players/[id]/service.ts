import {type ReplaysQuery} from "$lib/api/bodies";
import services from "$lib/api/services";

export type ReplayQueryKind = "allReplays" | "wonReplays" | "lostReplays";

export const replayQueryOptions: { label: string, value: ReplayQueryKind }[] = [
    { label: "All Games", value: "allReplays" },
    { label: "Won Games", value: "wonReplays" },
    { label: "Lost Games", value: "lostReplays" },
];
export const nonDefaultQueries = replayQueryOptions
    .filter(e => e.value !== "allReplays")
    .map((tab) => tab.value);

export async function loadReplays(replayQueryKind: ReplayQueryKind, userId: number, lastId: number | undefined) {
    const replaysQuery: ReplaysQuery = {
        afterId: lastId ? String(lastId) : undefined,
    };
    if (replayQueryKind === "allReplays") {
        replaysQuery.userId = String(userId);
    } else if (replayQueryKind === "wonReplays") {
        replaysQuery.winnerId = String(userId);
    } else if (replayQueryKind === "lostReplays") {
        replaysQuery.loserId = String(userId);
    }
    return await services.getReplays(replaysQuery);
}