<script lang="ts">
	import Banner from '$lib/Banner.svelte';
	import Board from '$lib/components/chess/Board.svelte';
	import RightIcon from '$lib/components/icons/RightIcon.svelte';
	import LeftIcon from '$lib/components/icons/LeftIcon.svelte';
	import FlipIcon from '$lib/components/icons/FlipIcon.svelte';
	import { createMessage } from '$lib/services/error';
	import { getNotificationsContext } from '$lib/services/context';
	import MoveList from '$lib/components/chess/MoveList.svelte';
	import ReplayPanel from '$lib/components/user/ReplayPanel.svelte';
	import { type ChessBoard, type ChessGame, type MoveStep, type PieceMove } from '$lib/api/messages';
	import type { Hex, ReplayModel } from '$lib/api/model';
	import services from '$lib/api/services';
	import { createMoveState } from '$lib/state/move.svelte';
	import { getNewSelection, isMoveValid, mapHexagonList, moveBoardPiece, newMove, NoSelection, type Selection } from '$lib/services/chess';
	import { newGame } from '$lib/services/chess.js';
	import { makeMoveDyn } from '$lib/services/wasm';

	export interface ReplayProps {
		replay: ReplayModel;
	}

	const { data }: { data: ReplayProps } = $props();
	const { replay } = $derived(data);

	const { addNotification } = getNotificationsContext();

	let moveState = createMoveState();
	let subGameIndex: number | undefined = undefined;

	interface MoveElem {
		pm: PieceMove;
		moves: {pm: PieceMove, game: ChessGame}[];
		game?: ChessGame;
	}

	let moveList: MoveElem[] = $state([]);
	let initialBoard: ChessBoard | undefined = $state(undefined);

	let selection: Selection = $state(NoSelection);

	async function initMoveList(replayId: string) {
		const [data, err] = await services.getReplayMoveHistory(replayId);
		if (data) {
			const moveSteps = data.moveSteps;
			initialBoard = data.initialBoard;

			for (const step of moveSteps) {
				if (step?.move) {
					moveList.push({pm: step.move, game: step.game, moves: []});
				}
			}
		} else {
			const message = 'Failed to load replay move list: ' + createMessage(err);
			addNotification({ type: 'string', message, isSuccess: false });
		}
	}

	$effect(() => {
		initMoveList(String(replay.id));
	});

	$effect(() => {
		moveState.updateMoveCount(moveList.length);
	});

	const game = $derived.by(() => {
		let moveListTemp: {game?: ChessGame}[] = moveList;
		if (subGameIndex !== undefined) {
			moveListTemp = moveList[subGameIndex]?.moves;
		}
		return moveState.value.moveIndex !== undefined
			? moveListTemp[moveState.value.moveIndex]?.game
			: newGame(initialBoard);
	});

	const prevMove = $derived.by(() =>
		moveList.length > 0 && moveState?.value.moveIndex !== undefined
			? moveList[moveState.value.moveIndex].pm
			: undefined);

	async function onPieceMove(from: Hex, to: Hex) {
		let isValid = isMoveValid(game, from, to);
		const isSameHex = from.file == to.file && from.rank == to.rank;

		if (isSameHex || !isValid || moveState.value.moveIndex === undefined) {
			return;
		}

		const tempGame = await makeMoveDyn($state.snapshot(game?.board), {from, to});

		const moves = moveList[moveState.value.moveIndex].moves;
		moves.push({ pm: newMove(from, to), game: tempGame });

		subGameIndex = moveState.value.moveIndex;
		moveState.selectMove(0);
	}

	function onSelectMove(index: number) {
		subGameIndex = undefined;
		moveState.selectMove(index);
	}
</script>

<svelte:head>
	<title>Replay - Hexchess</title>
</svelte:head>
<Banner />
<div class="center-horizontal-container">
	<div class="center-vertical-container" style="align-items: stretch;">
		<Board
			board={game?.board}
			potentialMoves={mapHexagonList(selection.potentialMoves?.moves)}
			draggable="anyone"
			isWhitePerspective={moveState.value.isWhitePerspective}
			prevMove={prevMove}
			onSelectPiece={(hex) => selection = getNewSelection(game, hex)}
			onDeSelectPiece={() => selection = NoSelection}
			onDropPiece={(from, to) => onPieceMove(from, to)}
		/>
		<div class="side-table" style:width="300px">
			<ReplayPanel replay={replay} />
			<MoveList
				moveList={moveList}
				onSelectMove={onSelectMove}
				selectedMoveIndex={moveState.value.moveIndex}
			/>
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