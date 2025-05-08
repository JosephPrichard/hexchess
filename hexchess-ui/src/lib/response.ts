const responseCodes: { [key: string]: string } = {
    ERROR_PASSWORD_LENGTH: 'Password must be between 10 and 100 characters long.',
    ERROR_CONFIRM_PASSWORD: 'Passwords do not match.',
    ERROR_USERNAME_LENGTH: 'Username must be between 5 and 20 characters.',
    ERROR_UNSAFE_USERNAME: 'Username contains unsafe characters.',
    ERROR_DUPLICATE_USERNAME: 'This username is already taken.',
    ERROR_INVALID_LOGIN: 'Invalid username or password.',
    ERROR_REQUIRED_LOGIN: 'You must be logged in to access this feature.',
    ERROR_SESSION_EXPIRED: 'Your session has expired. Please log in again.',
    ERROR_INVALID_GAME: 'Cannot find a game for the given id.',
    ERROR_NOT_FOUND_CHALLENGE: 'The challenge does not exist anymore.',
    ERROR_INVALID_CHALLENGE_ACTION: 'This action is not allowed for the challenge.',
    ERROR_SELF_CHALLENGE: 'You cannot challenge yourself.',
    ERROR_DUPLICATE_CHALLENGE: 'You have already sent this challenge.',
    ERROR_UPDATE_CHALLENGE: 'You are not authorized to update this challenge.',
    SUCCESS_LOGIN: 'Login successful!',
    SUCCESS_REGISTER: 'Registration successful!',
    SUCCESS_UPDATE_PASSWORD: 'Password updated successfully!',
    SUCCESS_UPDATE_USER: 'User information updated successfully!',
    SUCCESS_CREATE_CHALLENGE: 'Challenge created successfully.',
    SUCCESS_UPDATE_CHALLENGE: 'Challenge updated successfully.',
    SUCCESS: 'Operation completed successfully.'
};

export function createMessage(code: string) {
    return responseCodes[code] || 'An unexpected error has occurred';
}
