package svc

import (
	"fmt"
	"github.com/orsinium-labs/enum"
)

type ReplayResult enum.Member[string]

var (
	ResultBuilder  = enum.NewBuilder[string, ReplayResult]()
	WhiteWin       = ResultBuilder.Add(ReplayResult{Value: "WHITE_WINS"})
	BlackWin       = ResultBuilder.Add(ReplayResult{Value: "BLACK_WINS"})
	Draw           = ResultBuilder.Add(ReplayResult{Value: "DRAW"})
	ReplayResults  = ResultBuilder.Enum()
	ResultsStrings = ReplayResults.Values()
)

type ReplayCause enum.Member[string]

var (
	CauseBuilder  = enum.NewBuilder[string, ReplayCause]()
	Checkmate     = CauseBuilder.Add(ReplayCause{Value: "CHECKMATE"})
	Forfeit       = CauseBuilder.Add(ReplayCause{Value: "FORFEIT"})
	Stalemate     = CauseBuilder.Add(ReplayCause{Value: "STALEMATE"})
	ReplayCauses  = CauseBuilder.Enum()
	CausesStrings = ReplayResults.Values()
)

type GameMode enum.Member[string]

var (
	ModeBuilder          = enum.NewBuilder[string, GameMode]()
	ModeTimed1Plus0      = ModeBuilder.Add(GameMode{Value: "TIMED_1+0"})
	ModeTimed3Plus2      = ModeBuilder.Add(GameMode{Value: "TIMED_3+2"})
	ModeTimed15Plus10    = ModeBuilder.Add(GameMode{Value: "TIMED_15+10"})
	ModeCorrespondence1  = ModeBuilder.Add(GameMode{Value: "CORRESPONDENCE_1"})
	ModeCorrespondence7  = ModeBuilder.Add(GameMode{Value: "CORRESPONDENCE_7"})
	ModeCorrespondence14 = ModeBuilder.Add(GameMode{Value: "CORRESPONDENCE_14"})
	Modes                = ModeBuilder.Enum()
	ModesValues          = Modes.Members()
	ModesStrings         = Modes.Values()
)

type Color enum.Member[string]

var (
	ColorBuilder  = enum.NewBuilder[string, Color]()
	White         = ColorBuilder.Add(Color{Value: "WHITE"})
	Black         = ColorBuilder.Add(Color{Value: "BLACK"})
	Random        = ColorBuilder.Add(Color{Value: "RANDOM"})
	Colors        = ColorBuilder.Enum()
	ColorsStrings = Colors.Values()
)

type OneOfError struct {
	Expected []string
	Actual   string
}

func (err *OneOfError) Error() string {
	return fmt.Sprintf("expected one of %v, got %v", err.Expected, err.Actual)
}

func MakeReplayResultError(actual string) error {
	return &OneOfError{Expected: ResultsStrings, Actual: actual}
}

func MakeReplayCauseError(actual string) error {
	return &OneOfError{Expected: CausesStrings, Actual: actual}
}

func MakeGameModeError(actual string) error {
	return &OneOfError{Expected: ModesStrings, Actual: actual}
}

func MakeColorError(actual string) error {
	return &OneOfError{Expected: ColorsStrings, Actual: actual}
}
