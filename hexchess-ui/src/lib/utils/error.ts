import type { ServiceModel } from '../api/models';

export const codes = {
	// HTTP codes
	errorUnknown: 'ERROR_UNKNOWN',
	errorPasswordLength: 'ERROR_PASSWORD_LENGTH',
	errorConfirmPassword: 'ERROR_CONFIRM_PASSWORD',
	errorUsernameLength: 'ERROR_USERNAME_LENGTH',
	errorUnsafeUsername: 'ERROR_UNSAFE_USERNAME',
	errorInvalidParticipants: 'ERROR_INVALID_PARTICIPANTS',
	errorInvalidCountry: 'ERROR_INVALID_COUNTRY',
	errorDuplicateUsername: 'ERROR_DUPLICATE_USERNAME',
	errorInvalidLogin: 'ERROR_INVALID_LOGIN',
	errorRequiredLogin: 'ERROR_REQUIRED_LOGIN',
	errorTooManyLoginAttempts: 'ERROR_TOO_MANY_LOGIN_ATTEMPTS',
	errorSessionExpired: 'ERROR_SESSION_EXPIRED',
	errorNotFoundChallenge: 'ERROR_NOT_FOUND_CHALLENGE',
	errorInvalidChallengeAction: 'ERROR_INVALID_CHALLENGE_ACTION',
	errorSelfChallenge: 'ERROR_SELF_CHALLENGE',
	errorDuplicateChallenge: 'ERROR_DUPLICATE_CHALLENGE',
	errorUpdateChallenge: 'ERROR_UPDATE_CHALLENGE',
	errorInvalidRequest: 'ERROR_INVALID_REQUEST',
	errorNotFoundUser: 'ERROR_NOT_FOUND_USER',
	errorSearchLimit: 'ERROR_SEARCH_LIMIT',
	errorInvalidFEN: 'ERROR_INVALID_FEN',

	// WS codes
	errorFatal: 'ERROR_FATAL',
	errorMessageType: 'ERROR_MESSAGE_TYPE',
	errorTurn: 'ERROR_TURN',
	errorInvalidMove: 'ERROR_INVALID_MOVE',
	errorFinishedGame: 'ERROR_FINISHED_GAME',
	errorInvalidGame: 'ERROR_INVALID_GAME',
	errorExpiredGame: 'ERROR_EXPIRED_GAME',
};

export const messages: Record<string, string> = {
	// HTTP messages
	[codes.errorUnknown]: 'An unexpected error occurred.',
	[codes.errorPasswordLength]: 'Password must be between 10 and 100 characters long.',
	[codes.errorConfirmPassword]: 'Passwords do not match.',
	[codes.errorUsernameLength]: 'Username must be between 5 and 20 characters.',
	[codes.errorUnsafeUsername]: 'Username contains unsafe characters.',
	[codes.errorInvalidCountry]: 'The provided country is invalid.',
	[codes.errorInvalidParticipants]: 'Invalid participants provided for the challenge.',
	[codes.errorDuplicateUsername]: 'This username is already taken.',
	[codes.errorInvalidLogin]: 'Invalid username or password.',
	[codes.errorTooManyLoginAttempts]: 'Too many login attempts. Please try again later.',
	[codes.errorRequiredLogin]: 'You must be logged in to access this feature.',
	[codes.errorSessionExpired]: 'Your session has expired. Please log in again.',
	[codes.errorNotFoundChallenge]: 'The challenge does not exist anymore.',
	[codes.errorInvalidChallengeAction]: 'This action is not allowed for the challenge.',
	[codes.errorSelfChallenge]: 'You cannot challenge yourself.',
	[codes.errorDuplicateChallenge]: 'You have already sent this challenge.',
	[codes.errorUpdateChallenge]: 'You are not authorized to update this challenge.',
	[codes.errorInvalidRequest]: 'The api was malformed or contained invalid data.',
	[codes.errorNotFoundUser]: 'The provided user is invalid or does not exist.',
	[codes.errorSearchLimit]: 'The search limit has been reached.',
	[codes.errorInvalidFEN]: 'The provided FEN string is invalid.',

	// WS messages
	[codes.errorFatal]: 'A fatal error occurred. Please reconnect or try again later.',
	[codes.errorMessageType]: 'Invalid message type received.',
	[codes.errorTurn]: "It's not your turn.",
	[codes.errorInvalidMove]: 'That move is invalid. Please try again.',
	[codes.errorFinishedGame]: 'The game has already finished.',
	[codes.errorInvalidGame]: 'Cannot find a game for the given id.',
	[codes.errorExpiredGame]: 'The game has expired due to inactivity.',
};

export function makeMessage(error?: ServiceModel | string) {
	console.error(error);
	return messages[(typeof error === "string" ? error : error?.message) || ''] || 'An unexpected error has occurred';
}