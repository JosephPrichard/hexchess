package svc

import (
	"context"
	"hexchess-lib/serrors"
	"hexchess-svc/model"
	"log/slog"

	"google.golang.org/protobuf/proto"

	"github.com/redis/go-redis/v9"
)

func (services *HexchessServices) GetChats(ctx context.Context, gameID model.GameID, count int64) ([]model.Chat, error) {
	chatsZSet := fmtGameChatsZSet(services.redis, gameID)

	strList, err := services.redis.Primary.ZRevRange(ctx, chatsZSet, 0, count).Result()
	if err != nil {
		return nil, serrors.Wrap("get the first chats", err, "count", count)
	}

	chats := make([]model.Chat, 0, len(strList))
	for i, str := range strList {
		chat, err := model.UnmarshalChat([]byte(str))
		if err != nil {
			return nil, serrors.Wrap("unmarshal chat", err, "index", i, "gameID", gameID)
		}
		chats = append(chats, chat)
	}

	slog.InfoContext(ctx, "retrieved chess state chats", "chats", chats, "zSetName", chatsZSet)
	return chats, nil
}

func (services *HexchessServices) InsertChat(ctx context.Context, gameID model.GameID, chat model.Chat) error {
	bytes, err := proto.Marshal(model.SerializeChat(chat))
	if err != nil {
		return serrors.Wrap("marshal chat", err)
	}
	chatsZSet := fmtGameChatsZSet(services.redis, gameID)
	if err := services.redis.Primary.ZAdd(ctx, chatsZSet, redis.Z{Score: float64(chat.SentAt.UnixMilli()), Member: bytes}).Err(); err != nil {
		return serrors.Wrap("add chat to set", err, "chat", chat)
	}
	slog.InfoContext(ctx, "inserted state chat", "chat", chat, "zSetName", chatsZSet)
	return nil
}
