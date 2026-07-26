package svc

import (
	"context"
	"hexchess-svc/model"
	"hexchess-svc/utils/serrors"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

func (services *HexchessServices) GetChats(ctx context.Context, gameID model.GameID, count int64) ([]model.Chat, error) {
	chatsZSet := fmtGameChatsZSet(services.redis, gameID)

	strList, err := services.redis.PrimaryClient.ZRevRange(ctx, chatsZSet, 0, count).Result()
	if err != nil {
		return nil, serrors.New("get the first chats", err, "count", count)
	}

	chats := make([]model.Chat, 0, len(strList))
	for i, str := range strList {
		chat, err := model.UnmarshalChat([]byte(str))
		if err != nil {
			return nil, serrors.New("unmarshal chat", err, "index", i, "gameID", gameID)
		}
		chats = append(chats, chat)
	}

	slog.InfoContext(ctx, "retrieved chess state chats", "chats", chats, "zSetName", chatsZSet)
	return chats, nil
}

func (services *HexchessServices) InsertChat(ctx context.Context, gameID model.GameID, chat model.Chat) error {
	bytes, err := model.SerializeChat(chat).MarshalVT()
	if err != nil {
		return serrors.New("marshal chat", err)
	}
	chatsZSet := fmtGameChatsZSet(services.redis, gameID)
	if err := services.redis.PrimaryClient.ZAdd(ctx, chatsZSet, redis.Z{Score: float64(chat.SentAt.UnixMilli()), Member: bytes}).Err(); err != nil {
		return serrors.New("add chat to set", err, "chat", chat)
	}
	slog.InfoContext(ctx, "inserted state chat", "chat", chat, "zSetName", chatsZSet)
	return nil
}
