package pb

func MakeChatMessage(ID string, msg string) *GameOutput {
	return &GameOutput{
		GameId: ID,
		Value: &GameOutput_Chat{
			Chat: &Chat{Message: msg},
		},
	}
}
