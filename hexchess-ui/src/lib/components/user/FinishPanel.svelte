<script lang="ts">
	import { formatEloDiff, getReplayColors } from '$lib/utils/format.js';
	import type { FinishState, PlayerState } from '$lib/pb/messages';
	import { isGuestUser } from '$lib/api/models';

	interface FinishPanelProps {
		state: FinishState;
		whitePlayer: PlayerState;
		blackPlayer: PlayerState;
	}

	const { state, whitePlayer, blackPlayer }: FinishPanelProps = $props();

	const [winClass, loseClass] = getReplayColors(state.result);

	const result = $derived.by(() => {
		switch (state.result) {
		case 'WHITE_WINS':
			return "White Wins";
		case 'BLACK_WINS':
			return "Black Wins";
		case 'DRAW':
			return "Draw";
		default:
			return "-";
		}
	});

	const cause = $derived.by(() => {
		switch (state.cause) {
		case 'CHECKMATE':
			return "checkmate";
		case 'FORFEIT':
			return "forfeit";
		case 'STALEMATE':
			return "stalemate";
		default:
			return "-";
		}
	});

	const { winner, loser } = $derived.by(() => {
		let winner: PlayerState | undefined = undefined;

		winner = whitePlayer?.id === state.winId ? whitePlayer : undefined;
		winner = blackPlayer?.id === state.winId ? blackPlayer : undefined;

		let loser: PlayerState | undefined = undefined;
		loser = whitePlayer?.id === state.loseId ? whitePlayer : undefined;
		loser = blackPlayer?.id === state.loseId ? blackPlayer : undefined;

		if (!loser && !winner) {
			winner = whitePlayer;
			loser = blackPlayer;
		}
		return { winner, loser };
	});

	$inspect(state)
</script>

<div class="finish-state">
	<b class="result">
		{result}
	</b>
	<div class="cause">
		by {cause}
	</div>
	{#if winner && !isGuestUser(winner)}
		<div class="side-table-header-elem">
			<a href="/players/{winner.id}" class="text-ul">
				<b>{winner.name}</b>
			</a>
			<img class="flag" src="/flags/{winner.country}.png" alt="" />
			<span>({Math.round(winner.elo)})</span>
			<span class={winClass}>
			{formatEloDiff(state.winEloDiff)}
		</span>
		</div>
	{/if}
	{#if loser && !isGuestUser(loser)}
		<div class="side-table-header-elem">
			<a href="/players/{loser.id}" class="text-ul">
				<b>{loser.name}</b>
			</a>
			<img class="flag" src="/flags/{loser.country}.png" alt="" />
			<span>({Math.round(loser.elo)})</span>
			<span class={loseClass}>
			{formatEloDiff(state.loseEloDiff)}
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