<script lang="ts">
	import Banner from '$lib/Banner.svelte';
	import Board from '$lib/components/chess/Board.svelte';
	import RightIcon from '$lib/components/icons/RightIcon.svelte';
	import LeftIcon from '$lib/components/icons/LeftIcon.svelte';
	import FlipIcon from '$lib/components/icons/FlipIcon.svelte';
	import { makeMessage } from '$lib/utils/error';
	import { getNotificationsContext } from '$lib/utils/context';
	import MoveList from '$lib/components/chess/MoveList.svelte';
	import ReplayPanel from '$lib/components/user/ReplayPanel.svelte';
	import { type ChessGame, type PieceMove } from '$lib/api/messages';
	import type { Hex, ReplayModel } from '$lib/api/model';
	import services from '$lib/api/services';
	import { makeMoveState } from '$lib/state/move.svelte';
	import { makeMove, mapHexagons } from '$lib/utils/chess.js';
	import { makeMoveWasm } from '$lib/api/wasm';
	import TurnWrapper from '$lib/components/chess/TurnWrapper.svelte';
	import { makeMoveSelectionState } from '$lib/state/selection.svelte';

	export interface ReplayProps {
		replay: ReplayModel;
	}

	const { data }: { data: ReplayProps } = $props();
	const { replay } = $derived(data);

	const { addNotification } = getNotificationsContext();

	let moveState = makeMoveState();
	const selectionState = makeMoveSelectionState();

	let subGameIndex: number | undefined = undefined;

	interface ReplayMove {
		pm?: PieceMove
		notation: string
		game?: ChessGame;
	}

	type ReplayMoveRoot = ReplayMove & {moves: ReplayMove[]};

	let moveList: ReplayMoveRoot[] = $state([]);
	let initialGame: ChessGame | undefined = undefined;

	async function initMoveList(replayId: string) {
		const [data, err] = await services.getReplayMoveHistory(replayId);
		if (data) {
			initialGame = data.initialGame;
			moveList = data.steps.map(step => ({ pm: step.pm, notation: step.notMove, game: step.game, moves: [] }));
		} else {
			const message = 'Failed to load replay move list: ' + makeMessage(err);
			addNotification({ type: 'string', message, isSuccess: false });
		}
	}

	$effect(() => {
		initMoveList(String(replay.id));
	});

	$effect(() => {
		moveState.updateMoveCount(moveList.length);
	});

	function getGame(moveList: ReplayMoveRoot[], index?: number): [ChessGame | undefined, ReplayMove[]] {
		let moves: ReplayMove[] = moveList;
		if (subGameIndex !== undefined) {
			moves = moveList[subGameIndex]?.moves;
		}
		const game = index !== undefined ? moves[index]?.game : initialGame
		return [game, moves];
	}

	const prevMove = $derived.by(() =>
		moveList.length > 0 && moveState?.value.moveIndex !== undefined
			? moveList[moveState.value.moveIndex].pm
			: undefined);

	// async function onPieceMove(from: Hex, to: Hex) {
	// 	if (moveState.value.moveIndex === undefined)
	// 		return;
	//
	// 	const [game, moves] = getGame(moveList, moveState.value.moveIndex);
	//
	// 	const nextGame = await makeMoveWasm($state.snapshot(game), {from, to});
	// 	if (nextGame !== undefined) {
	// 		moves.push({ pm: makeMove(from, to), notation: "", game: nextGame });
	//
	// 		subGameIndex = moveState.value.moveIndex;
	// 		moveState.selectMove(0);
	// 	}
	// }

	async function onSelectMove(index: number) {
		subGameIndex = undefined;
		moveState.selectMove(index);
		onDeSelect();
	}

	function onSelect(hex: Hex) {
		selectionState.select(game, hex);
	}

	function onDeSelect() {
		selectionState.deSelect();
	}

	const [game, _] = $derived.by(() => getGame(moveList, moveState.value.moveIndex));
	const rootNotationList = $derived.by(() => moveList.map(move => move.notation));
	const potentialMoves = $derived.by(() => mapHexagons(selectionState.value.potentialMoves?.moves));
</script>

<svelte:head>
	<title>Replay - Hexchess</title>
</svelte:head>
<Banner />
<div class="center-horizontal-container">
	<div class="center-vertical-container" style="align-items: stretch;">
		<Board
			board={game?.board}
			fen={true}
			potentialMoves={potentialMoves}
			draggable="anyone"
			isWhitePerspective={moveState.value.isWhitePerspective}
			prevMove={prevMove}
			onSelectPiece={onSelect}
			onDeSelectPiece={onDeSelect}
		/>
		<div class="side-table side-table-capped" style:width="300px">
			<ReplayPanel replay={replay} />
			<TurnWrapper isWhitePerspective={moveState.value.isWhitePerspective} isWhiteTurn={game?.board?.isWhiteTurn} isEdged>
				<MoveList
					moveList={rootNotationList}
					onSelectMove={onSelectMove}
					selectedMoveIndex={moveState.value.moveIndex}
				/>
			</TurnWrapper>
			<div class="side-table-footer">
				<div class="move-table-nav-buttons">
					<button title="Previous Move" class="button-transparent" style:padding-top="5px" onclick={moveState.goLeft}>
						<LeftIcon />
					</button>
					<button title="Flip Board" class="button-transparent" style:padding-top="5px" onclick={moveState.flip}>
						<FlipIcon />
					</button>
					<button title="Next Move" class="button-transparent" style:padding-top="5px" onclick={moveState.goRight}>
						<RightIcon />
					</button>
				</div>
			</div>
		</div>
	</div>
</div>