<script lang="ts">
	import { formatCause, formatEloDiff, formatResult, getReplayColors } from '$lib/utils/format.js';
	import type { PlayerState, ReplayEntity, ReplayOutput } from '$lib/pb/messages';
	import { isGuestUser } from '$lib/api/models';

	interface FinishPanelProps {
		replay: ReplayEntity;
		whitePlayer: PlayerState;
		blackPlayer: PlayerState;
	}

	interface FinishView  {
		winClass: string;
		loseClass: string;
		winner: PlayerState;
		loser: PlayerState;
		result: string;
		cause: string;
		winEloDiff: number;
		loseEloDiff: number;
	}
	const { replay, whitePlayer, blackPlayer }: FinishPanelProps = $props();

	function getWinnerLoser(result: string): [PlayerState, PlayerState] {
		switch (result) {
		case 'BLACK_WINS':
			return [blackPlayer, whitePlayer];
		case 'WHITE_WINS':
		case 'DRAW':
		default:
			return [whitePlayer, blackPlayer];
		}
	}

	const view: FinishView = $derived.by(() => {
		const winEloDiff = replay.winEloDiff;
		const loseEloDiff = replay.loseEloDiff;

		const result = formatResult(replay.result);
		const cause = formatCause(replay.cause);
		const [winClass, loseClass] = getReplayColors(replay.result);
		const [winner, loser] = getWinnerLoser(replay.result);

		return { result, cause, winClass, loseClass, winner, loser, winEloDiff, loseEloDiff };
	});

	$inspect(replay)
</script>

<div class="finish-state">
	<b class="result">
		{view.result}
	</b>
	<div class="cause">
		by {view.cause}
	</div>
	{#if view.winner && !isGuestUser(view.winner)}
		<div class="side-table-header-elem">
			<a href="/players/{view.winner.id}" class="text-ul">
				<b>{view.winner.name}</b>
			</a>
			<img class="flag" src="/flags/{view.winner.country}.png" alt="" />
			<span>({Math.round(view.winner.elo)})</span>
			<span class={view.winClass}>
				{formatEloDiff(view.winEloDiff)}
			</span>
		</div>
	{/if}
	{#if view.loser && !isGuestUser(view.loser)}
		<div class="side-table-header-elem">
			<a href="/players/{view.loser.id}" class="text-ul">
				<b>{view.loser.name}</b>
			</a>
			<img class="flag" src="/flags/{view.loser.country}.png" alt="" />
			<span>({Math.round(view.loser.elo)})</span>
			<span class={view.loseClass}>
				{formatEloDiff(view.loseEloDiff)}
			</span>
		</div>
	{/if}
</div>

<style>
    .finish-state {
        text-align: center;
        margin-top: 10px;
        margin-bottom: 10px;
    }

	.result {
		font-size: 21px;
	}

	.cause {
		font-size: 14px;
	}
</style>