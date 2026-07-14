package perf

import (
	crand "crypto/rand"
	"fmt"
	"hexchess-svc/pb"
	"math/big"
	mrand "math/rand"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
)

const (
	GameIDChars          = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz123456789"
	GameIDPartitionChars = "abcdefghijklmnopqrstuvwxyz"
	GameIDLength         = 24

	TournamentStatus = "SCHEDULED"
	MaxCount         = 10000
)

func newGameID() (gameID string, partitionKey string) {
	gameIDBytes := make([]byte, GameIDLength)
	for i := range gameIDBytes {
		gameIDBytes[i] = randChar(GameIDChars)
	}

	lastByte := randChar(GameIDPartitionChars)
	gameIDBytes[len(gameIDBytes)-1] = lastByte

	return string(gameIDBytes), string(lastByte)
}

func randChar(str string) byte {
	n, err := crand.Int(crand.Reader, big.NewInt(int64(len(str))))
	if err != nil {
		panic("failed to generate random number: " + err.Error())
	}
	return str[n.Int64()]
}

func randRange[T interface{ ~int64 | ~int }](low T, high T) T {
	return T(mrand.Intn(int(high))) + low // range (low, high)
}

func selectInputTournamentKeys(state State) ([]string, error) {
	rows, err := state.PGPool.Query(state.Context,
		"SELECT tournament_key as tkey FROM tournaments WHERE status = $1 LIMIT $2;",
		TournamentStatus,
		MaxCount)
	if err != nil {
		return nil, fmt.Errorf("select tournament keys: %w", err)
	}
	defer rows.Close()

	eventRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[struct {
		TournamentKey string `db:"tkey"`
	}])

	var tournamentKeys []string
	for _, row := range eventRows {
		tournamentKeys = append(tournamentKeys, row.TournamentKey)
	}
	return tournamentKeys, err
}

// InputGenerator functions do not return errors but rather panic because all data is sytem originated and therefore a programmer error within this script
type InputGenerator struct {
	TournamentKeys []string
	MinUserID      int64
	MaxUserID      int64
}

func NewInputGenerator(state State) (InputGenerator, error) {
	tournamentKeys, err := selectInputTournamentKeys(state)
	if err != nil {
		return InputGenerator{}, fmt.Errorf("select input tournament keys: %w", err)
	}
	return InputGenerator{
		TournamentKeys: tournamentKeys,
	}, nil
}

func (gen InputGenerator) GenerateFinishGameInput() (string, []byte) {
	gameID, pkey := newGameID()

	board := &pb.ChessBoard{}
	var moves []*pb.HistMove

	bytes, err := proto.Marshal(&pb.FinishGameEvent{
		GameId:       gameID,
		WhitePlayer:  randRange(gen.MinUserID, gen.MaxUserID),
		BlackPlayer:  randRange(gen.MinUserID, gen.MaxUserID),
		GameMode:     "CORRESPONDENCE_1",
		ReplayResult: "WHITE_WINS",
		ReplayCause:  "FORFEIT",
		Board:        board,
		Moves:        moves,
	})
	if err != nil {
		panic(fmt.Sprintf("failed to generate finish game input: %v", err))
	}

	return pkey, bytes
}

func (gen InputGenerator) GenerateUpdtGameInput() (string, []byte) {
	gameID, pkey := newGameID()

	bytes, err := proto.Marshal(&pb.UpdtMetadataEvent{
		GameId:      gameID,
		WhitePlayer: randRange(gen.MinUserID, gen.MaxUserID),
		BlackPlayer: randRange(gen.MinUserID, gen.MaxUserID),
		Mode:        "CORRESPONDENCE_1",
		FirstColor:  "WHITE",
	})
	if err != nil {
		panic(fmt.Sprintf("failed to generate updt game input: %v", err))
	}

	return pkey, bytes
}

func (gen InputGenerator) GenerateAdvanceTournamentInput() []byte {
	bytes, err := proto.Marshal(&pb.AdvanceTournamentEvent{
		TournamentKey: gen.TournamentKeys[randRange(0, len(gen.TournamentKeys)-1)],
		EventId:       uuid.NewString(),
	})
	if err != nil {
		panic(fmt.Sprintf("failed to generate advance tournament input: %v", err))
	}
	return bytes
}
