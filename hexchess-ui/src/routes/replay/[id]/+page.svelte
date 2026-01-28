<script lang="ts">
	import Board from '$lib/components/chess/Board.svelte';
	import RightIcon from '$lib/components/icons/RightIcon.svelte';
	import LeftIcon from '$lib/components/icons/LeftIcon.svelte';
	import FlipIcon from '$lib/components/icons/FlipIcon.svelte';
	import { makeMessage } from '$lib/utils/error';
	import MoveList from '$lib/components/chess/MoveList.svelte';
	import ReplayPanel from '$lib/components/user/ReplayPanel.svelte';
	import { type ChessGame, type HistMove } from '$lib/pb/messages';
	import { type Hex, type ReplayModel } from '$lib/api/models';
	import services from '$lib/api/services';
	import { makeMoveState } from '$lib/state/move.svelte';
	import { makeSelectionState } from '$lib/state/selection.svelte';
	import Banner from '$lib/Banner.svelte';
	import PlayIcon from '$lib/components/icons/PlayIcon.svelte';
	import StopIcon from '$lib/components/icons/PauseIcon.svelte';
	import Timer from '$lib/components/chess/Timer.svelte';
	import { type CancelCountdown, gameAtStepIndex, getStepTimers, startCountdown, type TimerType } from './service';
	import TakenList from '$lib/components/chess/TakenList.svelte';
	import { moveElementHeight } from '$lib/components/chess/render';

	export interface ReplayProps {
		replay: ReplayModel;
	}

	const { data }: { data: ReplayProps } = $props();
	const { replay } = $derived(data);

	const move = makeMoveState();
	const selection = makeSelectionState();

	let initialGame: ChessGame | undefined = $state(undefined);
	let game: ChessGame | undefined = $state(undefined);
	let steps: HistMove[] = $state([]);
	let moveHistErr: string | undefined = $state(undefined);
	let isWhitePerspective = $state(false);

	let isPlaying = $state(false);
	let stepTimer: TimerType | undefined = $state(undefined);

	let countdown: CancelCountdown | undefined = undefined;
	let movesScroller: HTMLDivElement | undefined = $state(undefined);

	const stepIndex = $derived.by(() => move.state.moveIndex);
	const isWhiteTurn = $derived.by(() => game?.board?.isWhiteTurn);
	const prevMove = $derived.by(() => steps.length > 0 && stepIndex !== undefined ? steps[stepIndex] : undefined);
	const notList = $derived.by(() => steps.map(move => move.notation));
	const topTakenPieces = $derived.by(() => (isWhitePerspective ? game?.takenWhitePieces : game?.takenBlackPieces) ?? []);
	const bottomTakenPieces = $derived.by(() => (isWhitePerspective ? game?.takenBlackPieces : game?.takenWhitePieces) ?? []);

	async function initMoveList(replayId: string) {
		const [data, err] = await services.getReplayMoveHistory(replayId);
		if (data) {
			initialGame = data.initialGame;
			steps = data.steps;
		} else {
			moveHistErr = makeMessage(err);
		}
	}

	function stepCountdown(remaining: number) {
		if (!stepTimer) return;
		const isWhiteTurn = stepIndex === undefined || stepIndex % 2 == 0;
		if (!isWhiteTurn) {
			stepTimer.whiteTimerMs = remaining + stepTimer.endWhiteTimerMs;
		} else {
			stepTimer.blackTimerMs = remaining + stepTimer.endBlackTimerMs;
		}
	}

	function scrollToStep(stepIndex?: number) {
		if (movesScroller) {
			// find the top offset to scroll to in the moves scroller for a given index.
			const startTop =  movesScroller.scrollTop;
			const endTop = startTop + movesScroller.clientHeight;
			const nextTop = moveElementHeight * Math.floor((stepIndex ?? 0) / 2);

			if (nextTop > startTop && nextTop < endTop) return;

			movesScroller.scrollTo({ top: nextTop, behavior: 'smooth' });
		}
	}

	function onCompleteCountdown() {
		const diffMs = stepTimer?.diffMs ?? 0;
		if (diffMs > 0) {
			move.goRight();
			scrollToStep(stepIndex);
			beginCountdown();
		} else {
			cancelCountdown();
		}
	}

	function cancelCountdown() {
		if (countdown) countdown();
		countdown = undefined;
		isPlaying = false;
	}

	function beginCountdown() {
		onDeSelectPiece();
		if (!stepTimer) return;
		countdown = startCountdown(stepTimer.diffMs, stepCountdown, onCompleteCountdown);
	}

	function togglePlayPause() {
		isPlaying = !isPlaying;
		if (isPlaying && (stepTimer?.diffMs ?? 0) > 0) {
			beginCountdown();
		} else {
			cancelCountdown();
		}
	}

	async function onSelectMove(index: number) {
		cancelCountdown();
		move.selectMove(index);
	}

	function goLeft() {
		cancelCountdown();
		move.goLeft();
	}

	function goRight() {
		cancelCountdown();
		move.goRight();
	}

	const onSelectPiece = (hex: Hex) => selection.select(game, hex);
	const onDeSelectPiece = () => selection.deSelect();
	const flip = () => isWhitePerspective = !isWhitePerspective;

	$effect(() => {
		initMoveList(String(replay.id));
	});

	$effect(() => {
		move.updateMoveCount(steps.length);
	});

	$effect(() => {
		gameAtStepIndex(initialGame, steps, stepIndex).then(g => game = g);
	});

	$effect(() => {
		stepTimer = getStepTimers(stepIndex, steps, replay.mode);
	});
</script>

<svelte:head>
	<title>Replay - Hexchess</title>
</svelte:head>
<Banner />
{#if moveHistErr}
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
			draggable="none"
			isWhitePerspective={isWhitePerspective}
			prevMove={prevMove}
			selected={selection.state.hex}
			onSelectPiece={onSelectPiece}
			onDeSelectPiece={onDeSelectPiece}
		/>
		<div class="side-table side-table-capped" style:width="300px">
			<ReplayPanel replay={replay} />
			<div class="side-table-header">
				<div class="turn-circle" class:turn-circle-green={Boolean(isWhiteTurn) !== isWhitePerspective}></div>
				{isWhitePerspective ? "Black to move" : "White to move"}
			</div>
			<div class="side-table-header taken-wrapper">
				<TakenList myPieces={topTakenPieces} theirPieces={bottomTakenPieces}/>
			</div>
			<MoveList
				bind:containerElement={movesScroller}
				moveList={notList}
				onSelectMove={onSelectMove}
				selectedMoveIndex={move.state.moveIndex}
			/>
			<div class="side-table-header-bottom side-table-header-bottom-shadow taken-wrapper">
				<TakenList myPieces={bottomTakenPieces} theirPieces={topTakenPieces}/>
			</div>
			<div class="side-table-header-bottom">
				<div class="turn-circle" class:turn-circle-green={Boolean(isWhiteTurn) === isWhitePerspective}></div>
				{isWhitePerspective ? "White to move" : "Black to move"}
			</div>
			{#if stepTimer !== undefined}
				<div class="side-table-header-bottom replay-timers">
					<div class="replay-timer left">
						<Timer value={stepTimer.whiteTimerMs} size="sm" color="white"/>
					</div>
					<div class="replay-timer right">
						<Timer value={stepTimer.blackTimerMs} size="sm" color="black"/>
					</div>
				</div>
			{/if}
			<div class="side-table-footer">
				<div class="move-table-nav-buttons">
					{#if stepTimer !== undefined}
						{#if !isPlaying}
							<button title="Play" class="button-transparent" onclick={togglePlayPause}>
								<PlayIcon />
							</button>
						{:else}
							<button title="Stop" class="button-transparent" onclick={togglePlayPause}>
								<StopIcon />
							</button>
						{/if}
					{/if}
					<button title="Previous Move" class="button-transparent" onclick={goLeft}>
						<LeftIcon />
					</button>
					<button title="Flip Board" class="button-transparent" onclick={flip}>
						<FlipIcon color="rgb(100,100,100)"/>
					</button>
					<button title="Next Move" class="button-transparent" onclick={goRight}>
						<RightIcon />
					</button>
				</div>
			</div>
		</div>
	</div>
</div>

<style>
	.replay-timers {
        background-color: rgba(42, 42, 42);
		display: flex;
		padding: 0;
	}

	.replay-timer {
        flex: 0 0 calc(50% - 15px);
        padding-left: 10px;
        padding-right: 10px;
		margin-top: 10px;
		margin-bottom: 10px;
		border-radius: 4px;
	}

	.left {
		padding-left: 10px;
        padding-right: 5px;
	}

	.right {
		padding-right: 10px;
		padding-left: 5px;
	}
</style>