function createNotification(text, isSuccess) {
    const notifications = document.getElementById("notifications-box");

    const notificationText = document.createElement('div');
    notificationText.classList.add("notification-text");
    notificationText.appendChild(document.createTextNode(text));

    const xButton = document.createElement('span');
    xButton.classList.add("x-button");
    xButton.innerHTML = '&#10006;';

    const notification = document.createElement('div');
    notification.classList.add("notification");
    notification.classList.add(isSuccess ? "notification-green" : "notification-red");
    notification.appendChild(notificationText);
    notification.appendChild(xButton);

    xButton.onclick = () => notifications.removeChild(notification);

    notifications.append(notification);
    setTimeout(
        () => {
            if (notifications.contains(notification)) {
                notifications.removeChild(notification);
            }
            console.log("Deleted notification", text);
        },
        3000);

    console.log("Created notification", text);
}