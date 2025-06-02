export const codes = {
	errorUnknown: 'ERROR_UNKNOWN',
	errorPasswordLength: 'ERROR_PASSWORD_LENGTH',
	errorConfirmPassword: 'ERROR_CONFIRM_PASSWORD',
	errorUsernameLength: 'ERROR_USERNAME_LENGTH',
	errorUnsafeUsername: 'ERROR_UNSAFE_USERNAME',
	errorInvalidParticipants: 'ERROR_INVALID_PARTICIPANTS',
	errorDuplicateUsername: 'ERROR_DUPLICATE_USERNAME',
	errorInvalidLogin: 'ERROR_INVALID_LOGIN',
	errorRequiredLogin: 'ERROR_REQUIRED_LOGIN',
	errorSessionExpired: 'ERROR_SESSION_EXPIRED',
	errorNotFoundChallenge: 'ERROR_NOT_FOUND_CHALLENGE',
	errorInvalidChallengeAction: 'ERROR_INVALID_CHALLENGE_ACTION',
	errorSelfChallenge: 'ERROR_SELF_CHALLENGE',
	errorDuplicateChallenge: 'ERROR_DUPLICATE_CHALLENGE',
	errorUpdateChallenge: 'ERROR_UPDATE_CHALLENGE',
	errorInvalidRequest: 'ERROR_INVALID_REQUEST',
	errorInvalidGame: 'ERROR_INVALID_GAME',
	errorNotFoundUser: 'ERROR_NOT_FOUND_USER'
};

export const messages: Record<string, string> = {
	[codes.errorUnknown]: 'An unexpected error occurred.',
	[codes.errorPasswordLength]: 'Password must be between 10 and 100 characters long.',
	[codes.errorConfirmPassword]: 'Passwords do not match.',
	[codes.errorUsernameLength]: 'Username must be between 5 and 20 characters.',
	[codes.errorUnsafeUsername]: 'Username contains unsafe characters.',
	[codes.errorInvalidParticipants]: 'Invalid participants provided for the challenge.',
	[codes.errorDuplicateUsername]: 'This username is already taken.',
	[codes.errorInvalidLogin]: 'Invalid username or password.',
	[codes.errorRequiredLogin]: 'You must be logged in to access this feature.',
	[codes.errorSessionExpired]: 'Your session has expired. Please log in again.',
	[codes.errorNotFoundChallenge]: 'The challenge does not exist anymore.',
	[codes.errorInvalidChallengeAction]: 'This action is not allowed for the challenge.',
	[codes.errorSelfChallenge]: 'You cannot challenge yourself.',
	[codes.errorDuplicateChallenge]: 'You have already sent this challenge.',
	[codes.errorUpdateChallenge]: 'You are not authorized to update this challenge.',
	[codes.errorInvalidRequest]: 'The request was malformed or contained invalid data.',
	[codes.errorInvalidGame]: 'Cannot find a game for the given id.',
	[codes.errorNotFoundUser]: 'The provided user is invalid or does not exist.'
};

export function createMessage(code?: string | never) {
	return messages[code || ''] || 'An unexpected error has occurred';
}
