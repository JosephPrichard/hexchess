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
	errorInvalidMode: 'ERROR_INVALID_MODE',
	errorNotFoundUser: 'ERROR_NOT_FOUND_USER',
	errorNotFoundReplay: 'ERROR_NOT_FOUND_REPLAY',
	errorSearchLimit: 'ERROR_SEARCH_LIMIT',
	errorInvalidFEN: 'ERROR_INVALID_FEN',
	errorInvalidCount: 'ERROR_INVALID_COUNT',
	errorInvalidPage: 'ERROR_INVALID_PAGE',
	errorInvalidID: 'ERROR_INVALID_ID',
	errorInvalidJSON: 'ERROR_INVALID_JSON',
	errorInvalidTimeframe: 'ERROR_INVALID_TIMEFRAME',
	errorInvalidAction: 'ERROR_INVALID_ACTION',
	errorBioLength: 'ERROR_BIO_LENGTH',
	errorInvalidRounds: 'ERROR_INVALID_ROUNDS',
	errorNotFoundTournaments: 'ERROR_NOT_FOUND_TOURNAMENT',
	errorTournamentNotLobby: 'ERROR_NOT_LOBBY',
	errorTooManyParticipants: 'ERROR_TOO_MANY_PARTICIPANTS',
	errorInvalidCountdownState: 'ERROR_INVALID_COUNTDOWN_STATE',
	errorCountdownPermissions: 'ERROR_COUNTDOWN_PERMISSIONS',

	// WS codes
	errorFatal: 'ERROR_FATAL',
	errorMessageType: 'ERROR_MESSAGE_TYPE',
	errorTurn: 'ERROR_TURN',
	errorFfPlayer: 'ERROR_FORFEIT_PLAYER',
	errorInvalidMove: 'ERROR_INVALID_MOVE',
	errorFinishedGame: 'ERROR_FINISHED_GAME',
	errorStartedGame: 'ERROR_STARTED_GAME',
	errorInvalidGame: 'ERROR_INVALID_GAME',
	errorUndoCurrPlayer: 'ERROR_UNDO_CURR_PLAYER',
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
	[codes.errorInvalidMode]: 'The provided game mode is invalid.',
	[codes.errorNotFoundUser]: 'The provided user is invalid or does not exist.',
	[codes.errorNotFoundReplay]: 'The replay does not exist.',
	[codes.errorSearchLimit]: 'The search limit has been reached.',
	[codes.errorInvalidFEN]: 'The provided FEN string is invalid.',
	[codes.errorBioLength]: 'Bio must be between 0 and 160 characters.',
	[codes.errorInvalidCount]: 'The provided count parameter is invalid.',
	[codes.errorInvalidPage]: 'The provided page parameter is invalid.',
	[codes.errorInvalidID]: 'The provided id is invalid.',
	[codes.errorInvalidJSON]: 'The request body contains invalid JSON.',
	[codes.errorInvalidTimeframe]: 'The provided timeframe is invalid.',
	[codes.errorInvalidAction]: 'The provided action is invalid.',

	// WS messages
	[codes.errorFatal]: 'A fatal error occurred. Please reconnect or try again later.',
	[codes.errorMessageType]: 'Invalid message type received.',
	[codes.errorTurn]: "It's not your turn.",
	[codes.errorFfPlayer]: 'Cannot forfeit a game if you are not a player.',
	[codes.errorInvalidMove]: 'That move is invalid. Please try again.',
	[codes.errorFinishedGame]: 'The game has already finished.',
	[codes.errorStartedGame]: 'Cannot make a move on a game that hasn\'t started yet.',
	[codes.errorInvalidGame]: 'Cannot find a game for the given id.',
	[codes.errorUndoCurrPlayer]: 'Cannot propose a takeback if it is your turn.',
	[codes.errorExpiredGame]: 'The game has expired due to inactivity.',
};

function mapErr(code: string): string {
	return messages[code] || 'An unexpected error has occurred'
}

export function errorToArray(error?: ServiceModel): string[] {
	let messages: string[] = [];
	if (typeof error?.errors === "string") {
		messages = [error.errors];
	} else if (typeof error?.errors === "object") {
		messages = Object.values(error.errors);
	}
	return messages.map(message => mapErr(message));
}

export function makeMessage(error?: ServiceModel | string): string {
	console.error(error);

	const codes: string[] = [];
	if (typeof error === "string") {
		codes.push(error)
	} else if (typeof error?.errors === "string") {
		codes.push(error.errors)
	} else if (typeof error?.errors === "object") {
		for (const key in error.errors) {
			codes.push(error.errors[key])
		}
	}
	return codes
		.filter((value: string, index: number, array: string[]) => array.indexOf(value) === index) // distinct codes
		.map(code => mapErr(code))
		.join('\n');
}