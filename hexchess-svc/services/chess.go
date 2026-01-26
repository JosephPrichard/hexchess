package svc

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/go-cmp/cmp/cmpopts"
	"hexchess-svc/chess"
	"log/slog"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
)

type UndoState struct {
	UndoID int64
}

type EndKind int

const (
	NotEnded EndKind = iota
	Aborted
	Finished
)

type EndState struct {
	Kind        EndKind
	WinEloDiff  int64
	LoseEloDiff int64
	Cause       ReplayCause
	Result      ReplayResult
}

type ChessState struct {
	ChessMeta
	UndoState
	EndState     EndState
	InitialBoard chess.Board
	Game         chess.Game
}

func (s *ChessState) HasBothPlayers() bool {
	return s.WhitePlayer.Present && s.BlackPlayer.Present
}

func (s *ChessState) IsEitherPlayer(player PlayerState) bool {
	return s.WhitePlayer.IsSame(player) || s.BlackPlayer.IsSame(player)
}

type ChessMeta struct {
	ID          string      `json:"id"`
	WhitePlayer PlayerState `json:"whitePlayer"`
	BlackPlayer PlayerState `json:"blackPlayer"`
	FirstColor  Color       `json:"firstColor"`
	Mode        GameMode    `json:"mode"`
	Touch       time.Time   `json:"touch"`
}

var ChessMetaCmpOpt = cmpopts.IgnoreFields(ChessMeta{}, "Touch")

type StateSetup struct {
	ID           string
	Mode         GameMode
	FirstColor   Color
	White        PlayerState
	Black        PlayerState
	InitialBoard *chess.Board
	Game         *chess.Game
	FinishState  EndState
	UndoState    UndoState
}

func MakeChess(s StateSetup) ChessState {
	board := chess.MakeStartBoard()
	if s.InitialBoard != nil {
		board = *s.InitialBoard
	}
	game := chess.Game{Board: board}
	if s.Game != nil {
		game = *s.Game
	}
	state := ChessState{
		InitialBoard: board,
		Game:         game,
		UndoState:    s.UndoState,
		ChessMeta: ChessMeta{
			ID:          s.ID,
			FirstColor:  s.FirstColor,
			Mode:        s.Mode,
			Touch:       time.UnixMilli(0),
			WhitePlayer: s.White,
			BlackPlayer: s.Black,
		},
		EndState: s.FinishState,
	}
	return state
}

var ErrNoMoveUndo = errors.New("no move to undo")

func (s *ChessState) Undo() error {
	if len(s.Game.Moves) == 0 {
		return ErrNoMoveUndo
	}
	index := len(s.Game.Moves) - 2 // last element minus one.
	game, err := chess.JumpMoveIndex(s.InitialBoard, s.Game.Moves, index)
	if game != nil {
		s.Game = game.DeepCopy()
	}
	return err
}

func (s *ChessState) CurrPlayer() PlayerState {
	if s.Game.Board.IsWhiteTurn {
		return s.WhitePlayer
	}
	return s.BlackPlayer
}

func (s *ChessState) DeepCopy() ChessState {
	s2 := ChessState{
		Game:         s.Game.DeepCopy(),
		UndoState:    s.UndoState,
		InitialBoard: s.InitialBoard,
		ChessMeta: ChessMeta{
			ID:         s.ID,
			FirstColor: s.FirstColor,
			Mode:       s.Mode,
			Touch:      s.Touch,
		},
		EndState: EndState{
			WinEloDiff:  s.EndState.WinEloDiff,
			LoseEloDiff: s.EndState.LoseEloDiff,
			Cause:       s.EndState.Cause,
			Result:      s.EndState.Result,
		},
	}
	s2.WhitePlayer = s.WhitePlayer
	s2.BlackPlayer = s.BlackPlayer
	return s2
}

func (s *State) makeUserGameZSet(id int64) string {
	return s.Redis.GamesZSet + "/user_" + strconv.Itoa(int(id))
}

// IsGameAccessible we can just treat any inability to validate that the game exists as it "not existing", the client will just show a "404".
func (s *State) IsGameAccessible(ctx context.Context, id string) bool {
	//if err := s.ExpireChessStates(ctx, s.Redis.GamesZSet); err != nil {
	//	return false
	//}
	exists, err := s.Redis.Cache.Exists(ctx, makeGameKey(id)).Result()
	return err == nil && exists == 1
}

// AcquireChessLock is used to make sure only one client is ever allowed to write to a game ID at any time
// if the client fails to acquire the lock, the operation should fail rather than retry because the mutation will no longer be valid (underlying state will be swapped once freed)
// consider a situation where client 1 has fetched state A and is transforming it to state B. client 2 wants to transform state A to C. by the time client 1 is done writing, state C is outdated since it is based on an older version of A
func (s *State) AcquireChessLock(ctx context.Context, id string) bool {
	exists, err := s.Redis.Cache.SetNX(ctx, makeGameKey(id), "true", time.Second*5).Result()
	return err == nil && !exists // we acquired the lock, AND without errors.
}

func (s *State) ReleaseChessLock(ctx context.Context, id string) {
	err := s.Redis.Cache.Del(ctx, makeGameKey(id)).Err()
	if err != nil {
		slog.ErrorContext(ctx, "failed to release chess state lock", "id", id, "err", err)
	}
}

var ErrNoChessState = errors.New("no chess state")

func (s *State) GetChessState(ctx context.Context, id string) (*ChessState, error) {
	//if err := s.ExpireChessStates(ctx, s.Redis.GamesZSet); err != nil {
	//	return nil, fmt.Errorf("expire chess states: %w", err)
	//}
	fullID := makeGameKey(id)

	data, err := s.Redis.Cache.Get(ctx, fullID).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrNoChessState
	} else if err != nil {
		return nil, fmt.Errorf("get chess state in redis: %w", err)
	}

	state, err := UnmarshalChessState(data)
	if err != nil {
		return nil, fmt.Errorf("deserialize chess state: %w", err)
	}
	slog.InfoContext(ctx, "selected chess state", "key", fullID)
	return &state, nil
}

func (s *State) SetChessState(ctx context.Context, id string, state *ChessState) error {
	touch := time.Now()
	return s.SetChessStateAt(ctx, id, state, touch)
}

func makeGameKey(gameID string) string {
	return "game:" + gameID
}

func (s *State) makeGameChatsKey(gameKey string) string {
	return gameKey + "/" + s.GameChatsPostfix
}

const SetChessRetries = 5

func (s *State) SetChessStateAt(ctx context.Context, id string, state *ChessState, touch time.Time) error {
	state.Touch = touch
	touchSecs := float64(state.Touch.Unix())
	fullID := makeGameKey(id)

	b, err := proto.Marshal(SerializeChessState(state))
	if err != nil {
		return fmt.Errorf("marshal chess state: %w", err)
	}

	// retries with exponential backoffs are used to handle intermittent network issues to increase the chance that writes suceed
	backoff := 100 * time.Millisecond

	for range SetChessRetries {
		pipe := s.Redis.Cache.TxPipeline()
		pipe.Set(ctx, fullID, b, 0)

		if state.EndState.Kind != Aborted {
			pipe.ZAdd(ctx, s.Redis.GamesZSet, redis.Z{Score: touchSecs, Member: fullID})
			if state.WhitePlayer.Present {
				pipe.ZAdd(ctx, s.makeUserGameZSet(state.WhitePlayer.ID), redis.Z{Score: touchSecs, Member: fullID})
			}
			if state.BlackPlayer.Present {
				pipe.ZAdd(ctx, s.makeUserGameZSet(state.BlackPlayer.ID), redis.Z{Score: touchSecs, Member: fullID})
			}
		} else {
			pipe.ZRem(ctx, s.Redis.GamesZSet, fullID)
			if state.WhitePlayer.Present {
				pipe.ZRem(ctx, s.makeUserGameZSet(state.WhitePlayer.ID), fullID)
			}
			if state.BlackPlayer.Present {
				pipe.ZRem(ctx, s.makeUserGameZSet(state.BlackPlayer.ID), fullID)
			}
		}

		_, err = pipe.Exec(ctx)
		if err == nil {
			break
		}
		slog.WarnContext(ctx, "failed to set chess state in redis, retrying", "err", err)

		select {
		case <-time.After(backoff):
			backoff *= 2
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err != nil {
		return fmt.Errorf("set chess state in redis after %d retries: %w", SetChessRetries, err)
	}

	slog.InfoContext(ctx, "set chess state", "key", fullID, "touch", touch)
	return nil
}

type StateChat struct {
	Player  PlayerState `json:"player"`
	Message string      `json:"message"`
	SentAt  time.Time   `json:"time"`
}

func (s *State) GetStateChats(ctx context.Context, gameID string, count int64) ([]StateChat, error) {
	zSetName := s.makeGameChatsKey(makeGameKey(gameID))
	strList, err := s.Cache.ZRevRange(ctx, zSetName, 0, count).Result()
	if err != nil {
		return nil, fmt.Errorf("get the first %d chats: %w", count, err)
	}

	chats := make([]StateChat, 0, len(strList))
	for i, str := range strList {
		chat, err := UnmarshalChat([]byte(str))
		if err != nil {
			return nil, fmt.Errorf("unmarshal chat #%d for game=%s: %w", i, gameID, err)
		}
		chats = append(chats, chat)
	}

	slog.InfoContext(ctx, "retrieved chess state chats", "chats", chats, "zSetName", zSetName)
	return chats, nil
}

func (s *State) InsertStateChat(ctx context.Context, gameID string, chat StateChat) error {
	bytes, err := proto.Marshal(SerializeChat(chat))
	if err != nil {
		return fmt.Errorf("marshal chat: %w", err)
	}
	zSetName := s.makeGameChatsKey(makeGameKey(gameID))
	if err := s.Cache.ZAdd(ctx, zSetName, redis.Z{Score: float64(chat.SentAt.UnixMilli()), Member: bytes}).Err(); err != nil {
		return fmt.Errorf("add chat %v to zset: %w", chat, err)
	}
	slog.InfoContext(ctx, "inserted chess state chat", "chat", chat, "zSetName", zSetName)
	return nil
}

const GameExpireFinished = 1 * time.Hour

func (s *State) ExpireChessStates(ctx context.Context, gameZSetName string) error {
	expireBefore := time.Now().Add(-GameExpireFinished).Unix()

	keys, err := s.Redis.Cache.ZRangeByScore(ctx, gameZSetName, &redis.ZRangeBy{
		Min: "-inf",
		Max: strconv.FormatInt(expireBefore, 10),
	}).Result()
	if err != nil {
		return fmt.Errorf("retrieve expired states by range: %w", err)
	}
	if len(keys) == 0 {
		return nil
	}

	var chatsZSetNames []string

	pipe := s.Redis.Cache.TxPipeline()
	for _, key := range keys {
		chatZSetName := s.makeGameChatsKey(key)
		chatsZSetNames = append(chatsZSetNames, chatZSetName)
		pipe.Del(ctx, key)
		pipe.Del(ctx, chatZSetName)
	}
	pipe.ZRem(ctx, gameZSetName, keys)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("delete expired states: %w", err)
	}

	slog.InfoContext(ctx, "expired chess states", "gameZSetName", gameZSetName, "chatsZSetNames", chatsZSetNames, "keys", keys, "expireBefore", expireBefore)
	return nil
}

func (s *State) GetUserChessMetas(ctx context.Context, userID int64) ([]ChessMeta, error) {
	return s.GetUserChessMetasPaged(ctx, userID, 1, -1)
}

func (s *State) GetUserChessMetasPaged(ctx context.Context, userID int64, page, count int) ([]ChessMeta, error) {
	return s.GetChessMetas(ctx, s.makeUserGameZSet(userID), page, count)
}

func (s *State) GetAllChessMetas(ctx context.Context, page, count int) ([]ChessMeta, error) {
	return s.GetChessMetas(ctx, s.Redis.GamesZSet, page, count)
}

func (s *State) GetChessMetas(ctx context.Context, zSetName string, page, count int) ([]ChessMeta, error) {
	if page < 1 {
		page = 1
	}
	var start, stop int64
	if count >= 0 {
		start = int64((page - 1) * count)
		stop = start + int64(count) - 1
	} else {
		start = 0
		stop = -1
	}

	//if err := s.ExpireChessStates(ctx, zSetName); err != nil {
	//	return nil, fmt.Errorf("expire chess states: %w", err)
	//}

	elements, err := s.Redis.Cache.ZRevRange(ctx, zSetName, start, stop).Result()
	if err != nil {
		return nil, fmt.Errorf("retrieve chess ids by range: %w", err)
	}
	if len(elements) == 0 {
		return nil, nil
	}

	mgetList, err := s.Redis.Cache.MGet(ctx, elements...).Result()
	if err != nil {
		return nil, fmt.Errorf("get many chess states: %w", err)
	}

	views := make([]ChessMeta, 0, len(mgetList))
	for _, val := range mgetList {
		b, ok := val.(string)
		if !ok {
			return nil, fmt.Errorf("chess meta mget is not a string: %T", val)
		}
		cv, err := UnmarshalChessMeta([]byte(b))
		if err != nil {
			return nil, fmt.Errorf("unmarshal chess meta: %w", err)
		}
		views = append(views, cv)
	}

	slog.InfoContext(ctx, "retrieved chess meta views", "views", views, "zSetName", zSetName, "page", page)
	return views, nil
}

func (s *State) GetChessStateCount(ctx context.Context) (int64, error) {
	//if err := s.ExpireChessStates(ctx, s.Redis.GamesZSet); err != nil {
	//	return 0, err
	//}
	count, err := s.Redis.Cache.ZCard(ctx, s.Redis.GamesZSet).Result()
	if err != nil {
		return 0, fmt.Errorf("count chess states: %w", err)
	}
	slog.InfoContext(ctx, "selected chess count", "count", count)
	return count, nil
}
