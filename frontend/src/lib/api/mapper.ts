import type { ReplayOutput } from "$lib/pb/messages";
import type { Replay } from "./models";

export function nameMapIntoOptions<T extends string>(nameMap: Record<T, string>) {
	return Object.entries(nameMap)
		.map(([key, value]) => ({label: value as string, value: key as T}))
}

export function mapReplay(replay: ReplayOutput): Replay {
	return {
		id: Number(replay.id),
		whiteId: Number(replay.whiteId),
		blackId: Number(replay.blackId),
		whiteName: replay.whiteName,
		blackName: replay.blackName,
		whiteCountry: replay.whiteCountry,
		blackCountry: replay.blackCountry,
		whiteElo: replay.whiteElo,
		blackElo: replay.blackElo,
		playedOn: replay.playedOn,
		result: replay.result,
		cause: replay.cause,
		mode: replay.mode,
		whiteEloDiff: replay.whiteEloDiff,
		blackEloDiff: replay.blackEloDiff,
		winEloDiff: replay.winEloDiff,
		loseEloDiff: replay.loseEloDiff
	};
}
