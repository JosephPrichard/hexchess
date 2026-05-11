import {
    type GameMode,
    GameModeNameMap,
    type ReplayCause,
    ReplayCauseNameMap,
    type ReplayQuerySortKey, ReplayQuerySortKeyNameMap,
    type ReplayResult,
    ReplayResultNameMap,
} from "$lib/api/models";
import type {ReplaysQuery} from "$lib/api/services";

export type OptEnum<T> = T | "";

export const parseEnum = <T extends string>(value: string | null | undefined, nameMap: Record<T, string>): OptEnum<T> =>
    value !== "" && nameMap[value as T] !== undefined ? value as T : ""

export const expectEnum = <T extends string>(value: string | null | undefined, nameMap: Record<T, string>, fallback: T): T =>
    value !== "" && nameMap[value as T] !== undefined ? value as T : fallback

export const defaultOption = <T extends string>() =>
    ({ value: "" as T, label: "" });

export interface ReplaysSearchProps {
    whitename: string | null;
    blackname: string | null;
    winnername: string | null;
    losername: string | null;

    mode: OptEnum<GameMode>;
    result: OptEnum<ReplayResult>;
    cause: OptEnum<ReplayCause>;

    fromDate: string | null;
    toDate: string | null;

    sort: ReplayQuerySortKey | null;
}

export function mapReplaysPropsToQuery(props?: ReplaysSearchProps, cursors?: {afterId?: number, afterTurnCount?: number, afterRating?: number}): ReplaysQuery {
    const {afterId, afterTurnCount, afterRating} = cursors ?? {};
    return {
        whitename: String(props?.whitename ?? ""),
        blackname: String(props?.blackname ?? ""),
        winnername: String(props?.winnername ?? ""),
        losername: String(props?.losername ?? ""),

        fromDate: props?.fromDate ?? "",
        toDate: props?.toDate ?? "",
        mode: props?.mode,
        result: props?.result,
        cause: props?.cause,
        afterId: String(afterId ?? ""),
        afterTurnCount: String(afterTurnCount ?? ""),
        afterRating: String(afterRating ?? ""),

        sort: props?.sort ?? ""
    };
}

export function mapReplaysURLParamsToProps(params: URLSearchParams): ReplaysSearchProps {
    return {
        whitename: params.get("whitename"),
        blackname: params.get("blackname"),
        winnername: params.get("winnername"),
        losername: params.get("losername"),

        fromDate: params.get("fromDate"),
        toDate: params.get("toDate"),

        mode: parseEnum(params.get("mode"), GameModeNameMap),
        result: parseEnum(params.get("result"), ReplayResultNameMap),
        cause: parseEnum(params.get("cause"), ReplayCauseNameMap),
        sort: expectEnum(params.get("sort"), ReplayQuerySortKeyNameMap, "id"),
    };
}

export function mapReplaysPropsToURLParams(props: ReplaysSearchProps): URLSearchParams {
   const params = new URLSearchParams();
    for (const [key, value] of Object.entries(props)) {
        if (value) params.set(key, value);
    }
    return params;
}