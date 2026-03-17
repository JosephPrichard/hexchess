package svc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"google.golang.org/protobuf/proto"

	"hexchess-svc/chess"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/redis/go-redis/v9"
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

func (kind EndKind) isEnded() bool {
	return kind != NotEnded
}

type EndState struct {
	ReplayID    int64 // initialized on end state creation but not serialized
	Kind        EndKind
	WinEloDiff  int64
	LoseEloDiff int64
	Cause       ReplayCause
	Result      ReplayResult
}

type ChessState struct {
	ChessMeta
	UndoState
	EndState     EndKind
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
	EndState     EndKind
	UndoState    UndoState
}

func MakeChessState(s StateSetup) ChessState {
	board := chess.MakeStartBoard()
	if s.InitialBoard != nil {
		board = *s.InitialBoard
	}
	game := chess.Game{Board: board}
	if s.Game != nil {
		game = *s.Game
	}
	return ChessState{
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
		EndState: s.EndState,
	}
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
		EndState: s.EndState,
	}
	s2.WhitePlayer = s.WhitePlayer
	s2.BlackPlayer = s.BlackPlayer
	return s2
}

// IsGameAccessible we can just treat any inability to validate that the game exists as it "not existing", the client will just show a "404".
func (svc *Services) IsGameAccessible(ctx context.Context, id string) bool {
	gameKey := svc.Redis.MakeGameKey(id)

	exists, err := svc.Redis.Cache.Exists(ctx, gameKey).Result()

	return err == nil && exists == 1
}

// AcquireChessLock is used to make sure only one client is ever allowed to write to a game ID at any time
// if the client fails to acquire the lock, the operation should fail rather than retry because the mutation will no longer be valid (underlying state will be swapped once freed)
// consider a situation where client 1 has fetched state A and is transforming it to state B. client 2 wants to transform state A to C. by the time client 1 is done writing, state C is outdated since it is based on an older version of A
func (svc *Services) AcquireChessLock(ctx context.Context, id string) error {
	lockKey := svc.Redis.MakeGameKey(id) + "/lock"

	ok, err := svc.Redis.Cache.SetNX(ctx, lockKey, "true", time.Second*5).Result()

	acquired := err == nil && ok // we acquired the lock, AND without errors.
	if acquired {
		return nil
	} else {
		return ErrLockedGame
	}
}

func (svc *Services) ReleaseChessLock(ctx context.Context, id string) {
	lockKey := svc.Redis.MakeGameKey(id) + "/lock"

	if err := svc.Redis.Cache.Del(ctx, lockKey).Err(); err != nil {
		slog.ErrorContext(ctx, "failed to release chess state lock", "id", id, "err", err)
	}
}

var ErrNoChessState = errors.New("no chess state")

func (svc *Services) GetChessState(ctx context.Context, id string) (*ChessState, error) {
	gameKey := svc.Redis.MakeGameKey(id)

	data, err := svc.Redis.Cache.Get(ctx, gameKey).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrNoChessState
		}
		return nil, fmt.Errorf("get chess state in redis: %w", err)
	}

	state, err := UnmarshalChessState(data)
	if err != nil {
		return nil, fmt.Errorf("deserialize chess state: %w", err)
	}
	slog.InfoContext(ctx, "selected chess state", "key", gameKey)
	return &state, nil
}

func (svc *Services) SetChessStateNow(ctx context.Context, id string, state *ChessState) error {
	touch := time.Now()
	return svc.SetChessState(ctx, id, state, touch)
}

func (svc *Services) SetChessState(ctx context.Context, id string, state *ChessState, touch time.Time) error {
	state.Touch = touch
	touchSecs := float64(state.Touch.Unix())
	gameKey := svc.Redis.MakeGameKey(id)

	bytes, err := proto.Marshal(SerializeChessState(state))
	if err != nil {
		return fmt.Errorf("marshal chess s: %w", err)
	}

	pipe := svc.Redis.Cache.TxPipeline()
	pipe.Set(ctx, gameKey, bytes, 0)

	if state.EndState != Aborted {
		// keeps the game at the front of top of the sorted games sets on update
		pipe.ZAdd(ctx, svc.Redis.GamesZSet, redis.Z{Score: touchSecs, Member: gameKey})
		if state.WhitePlayer.Present {
			pipe.ZAdd(ctx, svc.Redis.GetUserGameZSet(state.WhitePlayer.ID), redis.Z{Score: touchSecs, Member: gameKey})
		}
		if state.BlackPlayer.Present {
			pipe.ZAdd(ctx, svc.Redis.GetUserGameZSet(state.BlackPlayer.ID), redis.Z{Score: touchSecs, Member: gameKey})
		}
	} else {
		// aborted games should be removed from sorted sets, although the game itself is technically accessible
		pipe.ZRem(ctx, svc.Redis.GamesZSet, gameKey)
		if state.WhitePlayer.Present {
			pipe.ZRem(ctx, svc.Redis.GetUserGameZSet(state.WhitePlayer.ID), gameKey)
		}
		if state.BlackPlayer.Present {
			pipe.ZRem(ctx, svc.Redis.GetUserGameZSet(state.BlackPlayer.ID), gameKey)
		}
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("set chess state in redis: %w", err)
	}

	slog.InfoContext(ctx, "set chess state", "key", gameKey, "touch", touch)
	return nil
}

type StateChat struct {
	Player  PlayerState `json:"player"`
	Message string      `json:"message"`
	SentAt  time.Time   `json:"sentAt"`
}

func (svc *Services) GetStateChats(ctx context.Context, gameID string, count int64) ([]StateChat, error) {
	chatsZSet := svc.Redis.GetGameChatsZSet(svc.Redis.MakeGameKey(gameID))
	strList, err := svc.Redis.Cache.ZRevRange(ctx, chatsZSet, 0, count).Result()
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

	slog.InfoContext(ctx, "retrieved chess state chats", "chats", chats, "zSetName", chatsZSet)
	return chats, nil
}

func (svc *Services) InsertStateChat(ctx context.Context, gameID string, chat StateChat) error {
	bytes, err := proto.Marshal(SerializeChat(chat))
	if err != nil {
		return fmt.Errorf("marshal chat: %w", err)
	}
	chatsZSet := svc.Redis.GetGameChatsZSet(svc.Redis.MakeGameKey(gameID))
	if err := svc.Redis.Cache.ZAdd(ctx, chatsZSet, redis.Z{Score: float64(chat.SentAt.UnixMilli()), Member: bytes}).Err(); err != nil {
		return fmt.Errorf("add chat %v to zset: %w", chat, err)
	}
	slog.InfoContext(ctx, "inserted state chat", "chat", chat, "zSetName", chatsZSet)
	return nil
}

const GameExpireFinished = 1 * time.Hour

func (svc *Services) ExpireChessStates(ctx context.Context, gameZSetName string) error {
	expireBefore := time.Now().Add(-GameExpireFinished).Unix()

	gameKeys, err := svc.Redis.Cache.ZRangeByScore(ctx, gameZSetName, &redis.ZRangeBy{
		Min: "-inf",
		Max: strconv.FormatInt(expireBefore, 10),
	}).Result()
	if err != nil {
		return fmt.Errorf("retrieve expired state by range: %w", err)
	}
	if len(gameKeys) == 0 {
		return nil
	}

	var chatsZSets []string

	pipe := svc.Redis.Cache.TxPipeline()
	for _, gameKey := range gameKeys {
		chatZSet := svc.Redis.GetGameChatsZSet(gameKey)
		chatsZSets = append(chatsZSets, chatZSet)
		pipe.Del(ctx, gameKey)
		pipe.Del(ctx, chatZSet)
	}
	pipe.ZRem(ctx, gameZSetName, gameKeys)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("delete expired state: %w", err)
	}

	slog.InfoContext(ctx, "expired chess state", "gameZSetName", gameZSetName, "chatsZSetNames", chatsZSets, "keys", gameKeys, "expireBefore", expireBefore)
	return nil
}

func (svc *Services) GetUserChessMetas(ctx context.Context, userID int64) ([]ChessMeta, error) {
	return svc.GetUserChessMetasPaged(ctx, userID, 1, -1)
}

func (svc *Services) GetUserChessMetasPaged(ctx context.Context, userID int64, page, count int) ([]ChessMeta, error) {
	return svc.GetChessMetas(ctx, svc.Redis.GetUserGameZSet(userID), page, count)
}

func (svc *Services) GetAllChessMetas(ctx context.Context, page, count int) ([]ChessMeta, error) {
	return svc.GetChessMetas(ctx, svc.Redis.GamesZSet, page, count)
}

func (svc *Services) GetChessMetas(ctx context.Context, zSetName string, page, count int) ([]ChessMeta, error) {
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

	chessKeys, err := svc.Redis.Cache.ZRevRange(ctx, zSetName, start, stop).Result()
	if err != nil {
		return nil, fmt.Errorf("retrieve chess ids by range: %w", err)
	}
	if len(chessKeys) == 0 {
		return nil, nil
	}

	mgetList, err := svc.Redis.Cache.MGet(ctx, chessKeys...).Result()
	if err != nil {
		return nil, fmt.Errorf("get many chess states: %w", err)
	}

	chessViews := make([]ChessMeta, 0, len(mgetList))
	for _, val := range mgetList {
		str, ok := val.(string)
		if !ok {
			slog.Warn("chess meta mget output is not a string", "type", fmt.Sprintf("%T", val))
			continue
		}
		view, err := UnmarshalChessMeta([]byte(str))
		if err != nil {
			return nil, fmt.Errorf("unmarshal chess meta: %w", err)
		}
		chessViews = append(chessViews, view)
	}

	slog.InfoContext(ctx, "retrieved chess meta views", "elements", chessKeys, "views", chessViews, "zSetName", zSetName, "page", page)
	return chessViews, nil
}

func (svc *Services) GetChessStateCount(ctx context.Context) (int64, error) {
	count, err := svc.Redis.Cache.ZCard(ctx, svc.Redis.GamesZSet).Result()
	if err != nil {
		return 0, fmt.Errorf("count chess state: %w", err)
	}
	slog.InfoContext(ctx, "selected chess count", "count", count)
	return count, nil
}
