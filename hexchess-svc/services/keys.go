package svc

import "strconv"

func (svc *Services) GetLeaderboardZSet(mode string) string {
	return svc.Redis.LeaderboardZSet + "/mode:" + mode
}

func (svc *Services) GetUserGameZSet(id int64) string {
	return svc.Redis.GamesZSet + "/user/" + strconv.Itoa(int(id))
}

func (svc *Services) MakeGameKey(gameID string) string {
	return "game/" + gameID
}

func (svc *Services) GetGameChatsZSet(gameKey string) string {
	return svc.Redis.GameChatsZSet + "/" + gameKey
}

func (svc *Services) MakeSessionKey(sessionID string) string {
	return "session/" + sessionID
}
