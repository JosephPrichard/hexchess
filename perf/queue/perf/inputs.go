package perf

import (
	"fmt"
	"perf-test-queue/pb"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

const GameIDChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz123456789"
const GameIDPartitionChars = "abcdefghijklmnopqrstuvwxyz"

const GameIDLength = 24

func newGameID() (gameID string, partitionKey string) {
	gameIDBytes := make([]byte, GameIDLength)
	for i := range gameIDBytes {
		gameIDBytes[i] = randChar(GameIDChars)
	}

	lastByte := randChar(GameIDPartitionChars)
	gameIDBytes[len(gameIDBytes)-1] = lastByte

	return string(gameIDBytes), string(lastByte)
}

// InputGenerator functions do not return errors but rather panic because all data is sytem originated and therefore a programmer error within this script
type InputGenerator struct {
	MinUserID      int64
	MaxUserID      int64
	TournamentKeys []string  
}

func (gen InputGenerator) GenerateFinishGameInput() (string, []byte) {
	gameID, pkey := newGameID()

	bytes, err := proto.Marshal(&pb.FinishGameEvent{
		GameId:       gameID,
		WhitePlayer:  randRange(gen.MinUserID, gen.MaxUserID),
		BlackPlayer:  randRange(gen.MinUserID, gen.MaxUserID),
		GameMode:     "CORRESPONDENCE_1",
		ReplayResult: "WHITE_WINS",
		ReplayCause:  "FORFEIT",
		// TODO add a large move list to test a long move history.
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
