package svc

import (
	"context"
	"fmt"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/model"
	"hexchess-svc/pb"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

func (svc *HexchessServices) GetStateChats(ctx context.Context, gameID string, count int64) ([]*pb.ChatMessage, error) {
	chatsZSet := fmtGameChatsZSet(svc.redis, fmtGameKey(svc.redis, gameID))
	strList, err := svc.redis.Cache.ZRevRange(ctx, chatsZSet, 0, count).Result()
	if err != nil {
		return nil, fmt.Errorf("get the first %d chats: %w", count, err)
	}

	pbChats := make([]*pb.ChatMessage, 0, len(strList))
	for i, str := range strList {
		pbChat := &pb.ChatMessage{}
		if err := proto.Unmarshal([]byte(str), pbChat); err != nil {
			return nil, fmt.Errorf("unmarshal chat #%d for game=%s: %w", i, gameID, err)
		}
		pbChats = append(pbChats, pbChat)
	}

	slog.InfoContext(ctx, "retrieved chess state chats", "chats", pbChats, "zSetName", chatsZSet)
	return pbChats, nil
}

func (svc *HexchessServices) InsertStateChat(ctx context.Context, gameID string, chat model.Chat) error {
	bytes, err := proto.Marshal(model.SerializeChat(chat))
	if err != nil {
		return fmt.Errorf("marshal chat: %w", err)
	}
	chatsZSet := fmtGameChatsZSet(svc.redis, fmtGameKey(svc.redis, gameID))
	if err := svc.redis.Cache.ZAdd(ctx, chatsZSet, redis.Z{Score: float64(chat.SentAt.UnixMilli()), Member: bytes}).Err(); err != nil {
		return fmt.Errorf("add chat %v to zset: %w", chat, err)
	}
	slog.InfoContext(ctx, "inserted state chat", "chat", chat, "zSetName", chatsZSet)
	return nil
}
