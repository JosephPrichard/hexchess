package web

import (
	svc "hexchess-svc/service"
	"net/url"
)

const (
	minPasswordLength = 11
	minUsernameLength = 5
	maxUsernameLength = 35
	maxBioLength      = 500
)

func isPasswordValid(password string) bool {
	return len(password) >= minPasswordLength
}

func validateRegisterBody(body RegisterBody) error {
	var respErr ResponseError
	if !isPasswordValid(body.Password) {
		respErr.Put("password", ErrHttpInvalidPassword)
	}
	if body.Password != body.ConfirmPassword {
		respErr.Put("confirmPassword", ErrHttpConfirmPassword)
	}
	if !isUsernameValid(body.Username) {
		respErr.Put("username", ErrHttpInvalidUsername)
	}
	return respErr.AsError()
}

func isUsernameValid(username string) bool {
	l := len(username)
	return l >= minUsernameLength && l <= maxUsernameLength
}

func isBioValid(bio string) bool {
	return len(bio) <= maxBioLength
}

func validateUpdatePasswordBody(body UpdatePasswordBody) error {
	var respErr ResponseError
	if !isPasswordValid(body.NewPassword) {
		respErr.Put("newPassword", ErrHttpInvalidPassword)
	}
	if body.NewPassword != body.ConfirmNewPassword {
		respErr.Put("confirmNewPassword", ErrHttpConfirmPassword)
	}
	return respErr.AsError()
}

func (api *API) validateUpdateUserBody(body UpdateUserBody) error {
	var respErr ResponseError
	if body.NewUsername != "" {
		if !isUsernameValid(body.NewUsername) {
			respErr.Put("newUsername", ErrHttpInvalidUsername)
		}
	}
	if body.NewBio != "" {
		if !isBioValid(body.NewBio) {
			respErr.Put("newBio", ErrHttpInvalidBio)
		}
	}
	if body.NewCountry != "" {
		if _, ok := api.staticData.validCountries[body.NewCountry]; !ok {
			respErr.Put("newCountry", ErrHttpInvalidCountry)
		}
	}
	return respErr.AsError()
}

func (api *API) getLeaderboardQuery(q url.Values) (LeaderboardArgs, error) {
	var respErr ResponseError
	page, err := intQueryDefault(q, "page", 1)
	if err != nil {
		respErr.Put("page", ErrHttpInvalidPage)
	}
	mode, ok := svc.GameModeEnums[q.Get("mode")]
	if !ok {
		respErr.Put("mode", ErrHttpInvalidMode)
	}
	args := LeaderboardArgs{Page: page, Mode: mode}
	return args, respErr.AsError()
}
