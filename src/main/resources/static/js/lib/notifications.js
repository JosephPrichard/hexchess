const messages = {
    "ERROR_PASSWORD_LENGTH": "Password must be between 10 and 100 characters long.",
    "ERROR_CONFIRM_PASSWORD": "Passwords do not match.",
    "ERROR_USERNAME_LENGTH": "Username must be between 5 and 20 characters.",
    "ERROR_UNSAFE_USERNAME": "Username contains unsafe characters.",
    "ERROR_DUPLICATE_USERNAME": "This username is already taken.",
    "ERROR_INVALID_LOGIN": "Invalid username or password.",
    "ERROR_REQUIRED_LOGIN": "You must be logged in to access this feature.",
    "ERROR_SESSION_EXPIRED": "Your session has expired. Please log in again.",
    "ERROR_NOT_FOUND_CHALLENGE": "The challenge does not exist anymore.",
    "ERROR_INVALID_CHALLENGE_ACTION": "This action is not allowed for the challenge.",
    "ERROR_SELF_CHALLENGE": "You cannot challenge yourself.",
    "ERROR_DUPLICATE_CHALLENGE": "You have already sent this challenge.",
    "ERROR_UPDATE_CHALLENGE": "You are not authorized to update this challenge.",
    "SUCCESS_LOGIN": "Login successful!",
    "SUCCESS_REGISTER": "Registration successful!",
    "SUCCESS_UPDATE_PASSWORD": "Password updated successfully!",
    "SUCCESS_UPDATE_USER": "User information updated successfully!",
    "SUCCESS_CREATE_CHALLENGE": "Challenge created successfully.",
    "SUCCESS_UPDATE_CHALLENGE": "Challenge updated successfully.",
    "SUCCESS": "Operation completed successfully."
};

function createNotification(message, isSuccess, timeout) {
    if (!timeout) {
        timeout = 3000;
    }

    const notifications = document.getElementById("notifications-box");

    const text = document.createElement('div');
    text.className = 'notification-text';
    text.innerHTML = message;

    const space = document.createElement('div');
    space.className = 'notification-space';

    const xButton = document.createElement('span');
    xButton.className = 'x-button';
    xButton.innerHTML = '&#10006;';

    const notification = document.createElement('div');
    notification.className = `notification ${isSuccess ? "notification-green" : "notification-red"}`;

    notification.appendChild(text);
    notification.appendChild(space);
    notification.appendChild(xButton);

    xButton.addEventListener('click', () => notifications.removeChild(notification));

    notifications.append(notification);
    requestAnimationFrame(() => notification.classList.add('visible')); // apply the css transition after the element is written to DOM

    setTimeout(
        () => {
            if (notifications.contains(notification)) {
                notification.classList.remove('visible')
                setTimeout(() => notifications.removeChild(notification), 250); // delete the element after css transition is finished
            }
            console.log("Deleted a notification", message);
        },
        timeout);

    console.log("Created a notification", message);
}