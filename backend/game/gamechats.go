package game

import (
	"context"
	"hexchess-svc/cache"
	"hexchess-svc/cloud"
	"hexchess-svc/database/query"
	"hexchess-svc/gameid"
	"hexchess-svc/utils/async"
	"hexchess-svc/utils/entropy"
	"hexchess-svc/utils/perf"
	"hexchess-svc/utils/serrors"
	"log/slog"
	"strconv"
	"time"

	"uuid"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/refresh"
	"github.com/redis/go-redis/v9"
)

type ChatService struct {
	redis    cache.Redis
	querier  query.Querier
	esClient *elasticsearch.TypedClient

	entropy    entropy.Generator
	dispatcher async.Dispatcher
}

func NewChatService(
	redis cache.Redis, querier query.Querier, esClient *elasticsearch.TypedClient, entropy entropy.Generator, dispatcher async.Dispatcher,
) *ChatService {
	return &ChatService{redis: redis, querier: querier, entropy: entropy, esClient: esClient, dispatcher: dispatcher}
}

func (service *ChatService) GetChats(ctx context.Context, gameID gameid.Identifier, count int64) (_ []Chat, err error) {
	perf.WithContext(ctx).LogFailable(&err)

	chatsZSet := cache.FmtGameChatsZSet(gameID)

	strList, err := service.redis.PrimaryClient.ZRevRange(ctx, chatsZSet, 0, count).Result()
	if err != nil {
		return nil, serrors.New("retrieve chats", err, "count", count)
	}

	chats := make([]Chat, 0, len(strList))
	for i, str := range strList {
		chat, err := UnmarshalChat([]byte(str))
		if err != nil {
			return nil, serrors.New("unmarshal chat", err, "index", i, "gameID", gameID)
		}
		chats = append(chats, chat)
	}

	slog.InfoContext(ctx, "retrieved chess state chats", "chats", chats, "zSetName", chatsZSet)
	return chats, nil
}

func (service *ChatService) InsertChat(ctx context.Context, gameID gameid.Identifier, chat Chat) (_ Chat, err error) {
	perf.WithContext(ctx).LogFailable(&err)

	if chat.ID == "" {
		chat.ID = uuid.NewV7().String()
	}

	bytes, err := SerializeChat(chat).MarshalVT()
	if err != nil {
		return Chat{}, serrors.New("marshal chat", err)
	}

	chatsZSet := cache.FmtGameChatsZSet(gameID)
	if err := service.redis.PrimaryClient.ZAdd(ctx, chatsZSet, redis.Z{Score: float64(chat.SentAt.UnixMilli()), Member: bytes}).Err(); err != nil {
		return Chat{}, serrors.New("add chat to set", err, "chat", chat)
	}

	slog.InfoContext(ctx, "inserted chess state chat", "chat", chat, "zSetName", chatsZSet)

	// fire and forget rather than eventual consistency - we're ok if a couple chats are missing from the search index
	service.dispatcher.Go(func() {
		service.insertSearchableChat(context.WithoutCancel(ctx), gameID, chat)
	})
	return chat, nil
}

type ChatDocument struct {
	ID         string    `json:"id"`
	GameID     string    `json:"gameID"`
	PlayerID   string    `json:"playerId"`
	PlayerName string    `json:"playerName"`
	Message    string    `json:"message"`
	CreatedAt  time.Time `json:"createdAt"`
}

func (service *ChatService) insertSearchableChat(ctx context.Context, gameID gameid.Identifier, chat Chat) {
	perf.WithContext(ctx).Log()

	chatDocument := ChatDocument{
		ID:         chat.ID,
		GameID:     gameID.String(),
		PlayerID:   strconv.FormatInt(chat.Player.ID, 10),
		PlayerName: chat.Player.Name,
		Message:    chat.Message,
		CreatedAt:  chat.SentAt,
	}

	_, err := service.esClient.Index(cloud.ChatsEsIndex).
		Id(chatDocument.ID).
		Request(chatDocument).
		Refresh(refresh.True).
		Do(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to insert searchable chat", "error", err, "searchableChat", chat)
		return
	}
	slog.InfoContext(ctx, "inserted searchable chat", "searchableChat", chat)
}
