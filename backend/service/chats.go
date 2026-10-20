package service

import (
	"context"
	"hexchess-svc/cache"
	"hexchess-svc/database/query"
	"hexchess-svc/model"
	"hexchess-svc/utils/entropy"
	"hexchess-svc/utils/errutil"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

type ChatService struct {
	redis   cache.Redis
	querier query.Querier
	entropy entropy.Generator
}

func NewChatService(redis cache.Redis, querier query.Querier, entropy entropy.Generator) *ChatService {
	return &ChatService{redis: redis, querier: querier, entropy: entropy}
}

func (service *ChatService) GetChats(ctx context.Context, gameID model.GameID, count int64) ([]model.Chat, error) {
	chatsZSet := cache.FmtGameChatsZSet(gameID)

	strList, err := service.redis.PrimaryClient.ZRevRange(ctx, chatsZSet, 0, count).Result()
	if err != nil {
		return nil, errutil.New("retrieve chats", err, "count", count)
	}

	chats := make([]model.Chat, 0, len(strList))
	for i, str := range strList {
		chat, err := model.UnmarshalChat([]byte(str))
		if err != nil {
			return nil, errutil.New("unmarshal chat", err, "index", i, "gameID", gameID)
		}
		chats = append(chats, chat)
	}

	slog.InfoContext(ctx, "retrieved chess state chats", "chats", chats, "zSetName", chatsZSet)
	return chats, nil
}

func (service *ChatService) InsertChat(ctx context.Context, gameID model.GameID, chat model.Chat) error {
	bytes, err := model.SerializeChat(chat).MarshalVT()
	if err != nil {
		return errutil.New("marshal chat", err)
	}

	chatsZSet := cache.FmtGameChatsZSet(gameID)
	if err := service.redis.PrimaryClient.ZAdd(ctx, chatsZSet, redis.Z{Score: float64(chat.SentAt.UnixMilli()), Member: bytes}).Err(); err != nil {
		return errutil.New("add chat to set", err, "chat", chat)
	}

	slog.InfoContext(ctx, "inserted state chat", "chat", chat, "zSetName", chatsZSet)
	return nil
}
