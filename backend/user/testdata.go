package user

import (
	"hexchess-svc/itest"
)

var TestUsers = []User{
	{
		ID:       1,
		Username: "user1",
		Country:  "us",
		Bio:      "",
		JoinedOn: itest.TimeNow.Local(),
	},
	{
		ID:       2,
		Username: "user2",
		Country:  "us",
		Bio:      "",
		JoinedOn: itest.TimeNow.Local(),
	},
}
