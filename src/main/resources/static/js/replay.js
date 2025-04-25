function initReplayPageGivenBoard(initialBoard) {
    var state = {
        boardCache: [],
        currentIndex: undefined,
        isFlipped: false,
        selectMoveElem: undefined
    };

    var moveList = JSON.parse($("#move-list-data").html());

    var $boardElement = $("#chess-board");
    var $moveListElement = $("#replay-move-list");

    function getBoard(index) {
        if (state.boardCache[index] !== undefined) {
            return state.boardCache[index];
        }
        var board = JSON.parse(JSON.stringify(initialBoard));
        for (var i = 0; i <= index; i++) {
            var move = moveList[i];
            var toRank = move.to.rank;
            var toFile = move.to.file;
            var fromRank = move.from.rank;
            var fromFile = move.from.file;

            board.pieces[toFile][toRank] = board.pieces[fromFile][fromRank];
            board.pieces[fromFile][fromRank] = piece.empty;
        }
        state.boardCache[index] = board;
        return board;
    }

    function handleOnMove(index, $moveElem) {
        var board = getBoard(index);
        state.currentIndex = index;
        displayChessBoard($boardElement[0], board, state.isFlipped);

        if (state.selectMoveElem) {
            $(state.selectMoveElem).removeClass("selected-move");
        }
        state.selectMoveElem = $moveElem;
        $moveElem.addClass("selected-move");
    }

    function onClickFlip() {
        state.isFlipped = !state.isFlipped;
        var board = state.currentIndex !== undefined ? getBoard(state.currentIndex) : initialBoard;
        displayChessBoard($boardElement[0], board, state.isFlipped);
    }

    function onClickLeft() {
        if (state.currentIndex === undefined) {
            state.currentIndex = 0;
        } else if (state.currentIndex > 0) {
            state.currentIndex--;
        } else {
            return;
        }
        handleOnMove(state.currentIndex, $("#move-" + state.currentIndex));
    }

    function onClickRight() {
        if (state.currentIndex === undefined) {
            state.currentIndex = 0;
        } else if (state.currentIndex < moveList.length - 1) {
            state.currentIndex++;
        } else {
            return;
        }
        handleOnMove(state.currentIndex, $("#move-" + state.currentIndex));
    }

    $("#flip-board-button").on("click", onClickFlip);
    $("#left-button").on("click", onClickLeft);
    $("#right-button").on("click", onClickRight);

    displayChessBoard($boardElement[0], initialBoard, state.isFlipped);
    displayMoveList($moveListElement[0], moveList, handleOnMove);
}

function initReplayPage() {
    $.getJSON("/static/initial-board", function(initialBoard) {
        initReplayPageGivenBoard(initialBoard);
    });
}
