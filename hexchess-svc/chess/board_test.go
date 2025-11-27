package chess

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBoard_Fen(t *testing.T) {
	board := InitialBoard()
	got := board.Fen()
	assert.Equal(t, "6/P5p/RP4pr/N1P3p1n/Q2P2p2q/BBB1P1p1bbb/K2P2p2k/N1P3p1n/RP4pr/P5p/6 w", got)
}

func TestParse_Fen(t *testing.T) {
	for _, test := range []struct {
		name     string
		fen      string
		expBoard Board
		expErr   error
	}{
		{
			name:     "valid FEN",
			fen:      "6/P5p/RP4pr/N1P3p1n/Q2P2p2q/BBB1P1p1bbb/K2P2p2k/N1P3p1n/RP4pr/P5p/6 w",
			expBoard: InitialBoard(),
		},
		{
			name: "valid FEN with many empty files",
			fen:  "6/K6/8/9/10/11/10/8k/8/7/6 w",
			expBoard: MakeEmptyBoard(
				NotMove{"b1", WhiteKing},
				NotMove{"h9", BlackKing},
			),
		},
		{
			name:   "not enough files",
			fen:    "6/P5p/RP4pr/N1P3p1n w",
			expErr: ErrFenInvalidFiles,
		},
		{
			name:   "too many files",
			fen:    "6/P5p/RP4pr/N1P3p1n/Q2P2p2q/BBB1P1p1bbb/K2P2p2k/N1P3p1n/RP4pr/P5p/6/6 w",
			expErr: ErrFenInvalidFiles,
		},
		{
			name:   "no turn indicator",
			fen:    "6/P5p/RP4pr/N1P3p1n/Q2P2p2q/BBB1P1p1bbb/K2P2p2k/N1P3p1n/RP4pr/P5p/6",
			expErr: ErrFenMissingTurn,
		},
		{
			name:   "not enough pieces in a file",
			fen:    "6/P5p/RP4pr/N1P3p1n/Q2P2p2q/1/K2P2p2k/N1P3p1n/RP4pr/P5p/6 w",
			expErr: errors.New("file 'f' requires 11 pieces"),
		},
		{
			name:   "too many pieces in a file",
			fen:    "5kK/P5p/RP4pr/N1P3p1n/Q2P2p2q/BBB1P1p1bbb/K2P2p2k/N1P3p1n/RP4pr/P5p/6 w",
			expErr: errors.New("a7 is out of bounds"),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseFen(test.fen)

			assert.Equal(t, test.expErr, err)
			assert.Equal(t, test.expBoard, got)
		})
	}
}
