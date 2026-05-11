package web

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
	return len(username) >= minUsernameLength && len(username) <= maxUsernameLength
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

func validateUpdateUserBody(static StaticData, body UpdateUserBody) error {
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
		if !static.validCountries[body.NewCountry] {
			respErr.Put("newCountry", ErrHttpInvalidCountry)
		}
	}
	return respErr.AsError()
}
