package svc

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"time"

	"hexchess-svc/chess"
)

func RandomMoveHistSeq(mode GameMode, game chess.Game, low int, hi int) ([]chess.HistMove, error) {
	randRange := func(min, max float64) float64 {
		return min + rand.Float64()*(max-min)
	}

	for range rand.Intn(low) + (low + hi) {
		game.InitPieceMoves()

		var pmsArr []chess.PieceMoves
		for _, pm := range game.GetCurrMoves() {
			if len(pm.Moves) > 0 {
				pmsArr = append(pmsArr, pm)
			}
		}
		if len(pmsArr) == 0 {
			break
		}

		pms := pmsArr[rand.Intn(len(pmsArr))]
		if len(pms.Moves) == 0 {
			return nil, fmt.Errorf("expected at least one move, got none for game: %v", game)
		}
		pm := chess.PieceMove{
			Piece: game.Board.Get(pms.From.File, pms.From.Rank),
			From:  pms.From,
			To:    pms.Moves[rand.Intn(len(pms.Moves))],
		}

		toPiece := game.Board.Get(pm.To.File, pm.To.Rank)
		fromPiece := game.Board.Get(pm.From.File, pm.From.Rank)
		if toPiece.IsKing() {
			return nil, errors.New("should never be allowed to make a move to the king")
		}
		if pm.From == pm.To {
			return nil, fmt.Errorf("expected from != to, got %v", pm)
		}
		if toPiece.SameColor(fromPiece) {
			return nil, fmt.Errorf("expected move to not be to piece of same color %v", pm)
		}

		hm := game.MakeMove(chess.Move{From: pm.From, To: pm.To, Promotion: chess.QueenPromotion})
		game.Moves = append(game.Moves, hm)
	}

	moveSeq := game.Moves

	whiteTimer := mode.TotalTime()
	blackTimer := mode.TotalTime()
	if mode.IsRealTime() {
		for moveIdx := range moveSeq {
			timeIncr := float64(mode.TimeIncrement().Milliseconds())
			incr := math.Max(timeIncr, 1000) * randRange(0.5, 1.5)
			if moveIdx%2 == 0 {
				whiteTimer -= time.Duration(incr) * time.Millisecond
			} else {
				blackTimer -= time.Duration(incr) * time.Millisecond
			}
			moveSeq[moveIdx].WhiteTimer = whiteTimer
			moveSeq[moveIdx].BlackTimer = blackTimer
		}
	}

	slog.Info("random move sequence", "len", len(moveSeq))
	return moveSeq, nil
}
