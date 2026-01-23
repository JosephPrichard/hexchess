<script lang="ts">
	import Board from '$lib/components/chess/Board.svelte';
	import RightIcon from '$lib/components/icons/RightIcon.svelte';
	import LeftIcon from '$lib/components/icons/LeftIcon.svelte';
	import FlipIcon from '$lib/components/icons/FlipIcon.svelte';
	import { makeMessage } from '$lib/utils/error';
	import MoveList from '$lib/components/chess/MoveList.svelte';
	import ReplayPanel from '$lib/components/user/ReplayPanel.svelte';
	import { type ChessGame, type HistMove, type NotMoveStep, type PieceMove } from '$lib/pb/messages';
	import { type Hex, type ReplayModel, TimedGameModes } from '$lib/api/models';
	import services from '$lib/api/services';
	import { makeMoveState } from '$lib/state/move.svelte';
	import TurnWrapper from '$lib/components/chess/TurnWrapper.svelte';
	import { makeSelectionState } from '$lib/state/selection.svelte';
	import Banner from '$lib/Banner.svelte';
	import PlayIcon from '$lib/components/icons/PlayIcon.svelte';
	import StopIcon from '$lib/components/icons/PauseIcon.svelte';
	import { gameAtMoveIndex } from '$lib/api/wasm';
	import { defaultBoard } from '$lib/utils/chess';

	interface MoveStep {
		notMove: string;
		pm?: PieceMove;
		hm?: HistMove;
	}

	export interface ReplayProps {
		replay: ReplayModel;
	}

	const { data }: { data: ReplayProps } = $props();
	const { replay } = $derived(data);

	const isRealTimeReplay = $derived(TimedGameModes.includes(replay.mode));

	const move = makeMoveState();
	const selection = makeSelectionState();

	let initialGame: ChessGame | undefined = $state(undefined);
	let game: ChessGame | undefined = $state(undefined);
	let moveList: MoveStep[] = $state([]);
	let moveListErr: string | undefined = $state(undefined);

	let isPlaying = $state(false);

	async function initMoveList(replayId: string) {
		const [data, err] = await services.getReplayMoveHistory(replayId);
		if (data) {
			initialGame = data.initialGame;
			moveList = data.steps.map((step) => ({ notMove: step.notMove, pm: step.pm, hm: step.hm }));
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

	const togglePlayPause = () => isPlaying = !isPlaying;

	async function onSelectMove(index: number) {
		move.selectMove(index);
		onDeSelect();
	}

	const onSelect = (hex: Hex) => selection.select(game, hex);
	const onDeSelect = () => selection.deSelect();


	$effect(() => {
		const moveIndex = move.state.moveIndex;
		if (moveIndex === undefined) {
			game = initialGame
			return;
		}
		gameAtMoveIndex(initialGame?.board || defaultBoard, moveList.map(m => m.hm), moveIndex).then(g => game = g);
	});

	const prevMove = $derived.by(() => {
		const moveIndex = move.state.moveIndex;
		return moveList.length > 0 && moveIndex !== undefined ?
			moveList[moveIndex - 1]?.pm :
			undefined;
	});
	const rootNotationList = $derived.by(() => moveList.map(move => move.notMove));
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
			{#if isRealTimeReplay}
				<div class="side-table-footer" style="padding: 5px">
					<div class="move-table-nav-buttons">
						{#if !isPlaying}
							<button title="Play" class="button-transparent" onclick={togglePlayPause}>
								<PlayIcon />
							</button>
						{:else}
							<button title="Stop" class="button-transparent" onclick={togglePlayPause}>
								<StopIcon />
							</button>
						{/if}
					</div>
				</div>
			{/if}
			<div class="side-table-footer">
				<div class="move-table-nav-buttons">
					<button title="Previous Move" class="button-transparent" onclick={move.goLeft}>
						<LeftIcon />
					</button>
					<button title="Flip Board" class="button-transparent" onclick={move.flip}>
						<FlipIcon />
					</button>
					<button title="Next Move" class="button-transparent" onclick={move.goRight}>
						<RightIcon />
					</button>
				</div>
			</div>
		</div>
	</div>
</div>