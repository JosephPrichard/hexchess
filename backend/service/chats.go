package svc

import (
	"context"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/lib/serrors"
	"hexchess-svc/model"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

func (services *HexchessServices) GetChats(ctx context.Context, gameID string, count int64) ([]model.Chat, error) {
	chatsZSet := fmtGameChatsZSet(services.redis, fmtGameKey(gameID))

	strList, err := services.redis.Cache.ZRevRange(ctx, chatsZSet, 0, count).Result()
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

func (services *HexchessServices) InsertChat(ctx context.Context, gameID string, chat model.Chat) error {
	bytes, err := proto.Marshal(model.SerializeChat(chat))
	if err != nil {
		return serrors.New("marshal chat", err)
	}
	chatsZSet := fmtGameChatsZSet(services.redis, fmtGameKey(gameID))
	if err := services.redis.Cache.ZAdd(ctx, chatsZSet, redis.Z{Score: float64(chat.SentAt.UnixMilli()), Member: bytes}).Err(); err != nil {
		return serrors.New("add chat to zset", err, "chat", chat)
	}
	slog.InfoContext(ctx, "inserted state chat", "chat", chat, "zSetName", chatsZSet)
	return nil
}
