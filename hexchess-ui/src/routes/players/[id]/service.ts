import services, {type ReplaysQuery} from "$lib/api/services";

export type ReplayQueryKind = "allReplays" | "wonReplays" | "lostReplays";

export const replayQueryOptions: { label: string, value: ReplayQueryKind }[] = [
    { label: "All Games", value: "allReplays" },
    { label: "Won Games", value: "lostReplays" },
    { label: "Lost Games", value: "wonReplays" },
];
export const nonDefaultQueries = replayQueryOptions
    .filter(e => e.value !== "allReplays")
    .map((tab) => tab.value);

export async function getReplay(replayQueryKind: ReplayQueryKind, lastId: number | undefined, userId: number | undefined) {
    const replaysQuery: ReplaysQuery = {
        afterId: lastId,
    };
    if (replayQueryKind === "allReplays") {
        replaysQuery.userId = userId;
    } else if (replayQueryKind === "wonReplays") {
        replaysQuery.winnerId = userId;
    } else if (replayQueryKind === "lostReplays") {
        replaysQuery.loserId = userId;
    }
    return await services.getReplays(replaysQuery);
}