<script lang="ts">
	import Board from '$lib/components/chess/Board.svelte';
	import RightIcon from '$lib/components/icons/RightIcon.svelte';
	import LeftIcon from '$lib/components/icons/LeftIcon.svelte';
	import FlipIcon from '$lib/components/icons/FlipIcon.svelte';
	import { makeMessage } from '$lib/utils/error';
	import { getNotificationsContext } from '$lib/utils/context';
	import MoveList from '$lib/components/chess/MoveList.svelte';
	import ReplayPanel from '$lib/components/user/ReplayPanel.svelte';
	import { type ChessGame, type PieceMove } from '$lib/pb/messages';
	import type { Hex, ReplayModel } from '$lib/api/models';
	import services from '$lib/api/services';
	import { makeMoveState } from '$lib/state/move.svelte';
	import { deserializeHexList } from '$lib/utils/chess.js';
	import TurnWrapper from '$lib/components/chess/TurnWrapper.svelte';
	import { makeSelectionState } from '$lib/state/selection.svelte';
	import Banner from '$lib/Banner.svelte';
	import Error from '$lib/Error.svelte';

	export interface ReplayProps {
		replay: ReplayModel;
	}

	const { data }: { data: ReplayProps } = $props();
	const { replay } = $derived(data);

	const { addNotification } = getNotificationsContext();

	let move = makeMoveState();
	const selection = makeSelectionState();

	interface ReplayMove {
		pm?: PieceMove
		notation: string
		game?: ChessGame;
	}

	type ReplayMoveRoot = ReplayMove & {moves: ReplayMove[]};

	let moveList: ReplayMoveRoot[] = $state([]);
	let moveListErr: string | undefined = $state(undefined);
	let initialGame: ChessGame | undefined = undefined;

	async function initMoveList(replayId: string) {
		const [data, err] = await services.getReplayMoveHistory(replayId);
		if (data) {
			initialGame = data.initialGame;
			moveList = data.steps.map(step => ({ pm: step.pm, notation: step.notMove, game: step.game, moves: [] }));
		} else {
			moveListErr = makeMessage(err);
		}
	}

	$effect(() => {
		initMoveList(String(replay.id));
	});

	$effect(() => {
		move.updateMoveCount(moveList.length);
	});

	async function onSelectMove(index: number) {
		move.selectMove(index);
		onDeSelect();
	}

	function onSelect(hex: Hex) {
		selection.select(game, hex);
	}

	function onDeSelect() {
		selection.deSelect();
	}

	const [game, prevMove] = $derived.by(() => {
		const index = move.state.moveIndex;
		const game = index !== undefined
			? moveList[index]?.game
			: initialGame;
		let prevMove = moveList.length > 0 && move?.state.moveIndex !== undefined
			? moveList[move.state.moveIndex].pm
			: undefined;
		return [game, prevMove];
	});
	const rootNotationList = $derived.by(() => moveList.map(move => move.notation));
</script>

<svelte:head>
	<title>Replay - Hexchess</title>
</svelte:head>
<Banner />
{#if moveListErr}
	<div class="bottom-right-error">
		Unable to retrieve replay's move sequence.
	</div>
{/if}
<div class="center-horizontal-container">
	<div class="center-vertical-container" style="align-items: stretch;">
		<Board
			board={game?.board}
			fen={true}
			potentialMoves={selection.getPotentialMoves()}
			draggable="anyone"
			isWhitePerspective={move.state.isWhitePerspective}
			prevMove={prevMove}
			onSelectPiece={onSelect}
			onDeSelectPiece={onDeSelect}
		/>
		<div class="side-table side-table-capped" style:width="300px">
			<ReplayPanel replay={replay} />
			<TurnWrapper isWhitePerspective={move.state.isWhitePerspective} isWhiteTurn={game?.board?.isWhiteTurn} isEdged>
				<MoveList
					moveList={rootNotationList}
					onSelectMove={onSelectMove}
					selectedMoveIndex={move.state.moveIndex}
				/>
			</TurnWrapper>
			<div class="side-table-footer">
				<div class="move-table-nav-buttons">
					<button title="Previous Move" class="button-transparent" style:padding-top="5px" onclick={move.goLeft}>
						<LeftIcon />
					</button>
					<button title="Flip Board" class="button-transparent" style:padding-top="5px" onclick={move.flip}>
						<FlipIcon />
					</button>
					<button title="Next Move" class="button-transparent" style:padding-top="5px" onclick={move.goRight}>
						<RightIcon />
					</button>
				</div>
			</div>
		</div>
	</div>
</div>