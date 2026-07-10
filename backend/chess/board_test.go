package chess

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoard_Fen(t *testing.T) {
	t.Parallel()

	board := InitialBoard()
	got := board.Fen()
	assert.Equal(t, "6/P5p/RP4pr/N1P3p1n/Q2P2p2q/BBB1P1p1bbb/K2P2p2k/N1P3p1n/RP4pr/P5p/6 w", got)
}

func TestParse_Fen(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		fen       string
		wantBoard Board
		wantErr   error
	}{
		{
			name:      "valid FEN",
			fen:       "6/P5p/RP4pr/N1P3p1n/Q2P2p2q/BBB1P1p1bbb/K2P2p2k/N1P3p1n/RP4pr/P5p/6 w",
			wantBoard: InitialBoard(),
		},
		{
			name: "valid FEN with many empty files",
			fen:  "6/K6/8/9/10/11/10/8k/8/7/6 w",
			wantBoard: NewEmptyBoard(true,
				Place{"b1", WhiteKing},
				Place{"h9", BlackKing},
			),
		},
		{
			name:    "not enough files",
			fen:     "6/P5p/RP4pr/N1P3p1n w",
			wantErr: ErrFenInvalidFiles,
		},
		{
			name:    "too many files",
			fen:     "6/P5p/RP4pr/N1P3p1n/Q2P2p2q/BBB1P1p1bbb/K2P2p2k/N1P3p1n/RP4pr/P5p/6/6 w",
			wantErr: ErrFenInvalidFiles,
		},
		{
			name:    "no turn indicator",
			fen:     "6/P5p/RP4pr/N1P3p1n/Q2P2p2q/BBB1P1p1bbb/K2P2p2k/N1P3p1n/RP4pr/P5p/6",
			wantErr: ErrFenMissingTurn,
		},
		{
			name:    "not enough pieces in a file",
			fen:     "6/P5p/RP4pr/N1P3p1n/Q2P2p2q/1/K2P2p2k/N1P3p1n/RP4pr/P5p/6 w",
			wantErr: errors.New("file 'f' requires 11 pieces"),
		},
		{
			name:    "too many pieces in a file",
			fen:     "5kK/P5p/RP4pr/N1P3p1n/Q2P2p2q/BBB1P1p1bbb/K2P2p2k/N1P3p1n/RP4pr/P5p/6 w",
			wantErr: errors.New("a7 is ext of bounds"),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseFen(test.fen)

			assert.Equal(t, test.wantErr, err)
			assert.Equal(t, test.wantBoard, got)
		})
	}
}

func TestUnmarshal_BoardJSON(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		fen       string
		wantBoard Board
		wantErr   error
	}{
		{
			name:      "valid board json",
			fen:       "\"6/P5p/RP4pr/N1P3p1n/Q2P2p2q/BBB1P1p1bbb/K2P2p2k/N1P3p1n/RP4pr/P5p/6 w\"",
			wantBoard: InitialBoard(),
		},
		{
			name:    "",
			fen:     "6/P5p/RP4pr/N1P3p1n/Q2P2p2q/1/K2P2p2k/N1P3p1n/RP4pr/P5p/6 w",
			wantErr: InvalidBoardJSONString,
		},
		{
			name:    "missing string quotes",
			fen:     "a6/P5p/RP4pr/N1P3p1n/Q2P2p2q/BBB1P1p1bbb/K2P2p2k/N1P3p1n/RP4pr/P5p/6 wb",
			wantErr: InvalidBoardJSONString,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var b Board
			err := b.UnmarshalJSON([]byte(test.fen))

			assert.Equal(t, test.wantErr, err)
			assert.Equal(t, test.wantBoard, b)
		})
	}
}

func TestEcho_BoardJSON(t *testing.T) {
	t.Parallel()

	inputBoard := InitialBoard()

	jsonBytes, err := json.Marshal(inputBoard)
	require.NoError(t, err)

	t.Logf("marshalled json board: %v", string(jsonBytes))

	var outputBoard Board
	err = json.Unmarshal(jsonBytes, &outputBoard)
	require.NoError(t, err)

	assert.Equal(t, outputBoard, inputBoard)
}
