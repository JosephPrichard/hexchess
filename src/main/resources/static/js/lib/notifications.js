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

function createNotification(text, isSuccess, timeout) {
    if (timeout === undefined) {
        timeout = 3000;
    }

    const notifications = document.getElementById("notifications-box");

    const notificationText = document.createElement('div');
    notificationText.classList.add('notification-text');
    notificationText.appendChild(document.createTextNode(text));

    const notificationSpace = document.createElement('div');
    notificationSpace.classList.add('notification-space');

    const xButton = document.createElement('span');
    xButton.classList.add('x-button');
    xButton.innerHTML = '&#10006;';

    const notification = document.createElement('div');
    notification.classList.add('notification');
    notification.classList.add(isSuccess ? "notification-green" : "notification-red");
    notification.appendChild(notificationText);
    notification.appendChild(notificationSpace);
    notification.appendChild(xButton);

    xButton.onclick = () => notifications.removeChild(notification);

    notifications.append(notification);
    setTimeout(() => notification.classList.add('visible'), 0); // apply the css transition after the element is written to DOM

    setTimeout(
        () => {
            if (notifications.contains(notification)) {
                notification.classList.remove('visible')
                setTimeout(() => notifications.removeChild(notification), 250); // delete the element after css transition is finished
            }
            console.log("Deleted notification", text);
        },
        timeout);

    console.log("Created notification", text);
}