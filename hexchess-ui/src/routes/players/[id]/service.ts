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

export async function getReplay(
    replayQueryKind: ReplayQueryKind,
    lastId: number | undefined,
    userId: number | undefined
) {
    const replaysQuery: ReplaysQuery = {
        afterId: lastId,
    };
    const replayQueryKindMap = {
        "allReplays": () => replaysQuery.userId = userId,
        "wonReplays": () => replaysQuery.winnerId = userId,
        "lostReplays": () => replaysQuery.loserId = userId,
    };
    replayQueryKindMap[replayQueryKind]();

    return await services.getReplays(replaysQuery);
}