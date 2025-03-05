function getSessionCookie() {
    const value = ('; ' + document.cookie).split(`; session=`).pop().split(';')[0];
    if (value === "") {
        return undefined;
    }
    const fields = value.substring(1, value.length - 1).split(",");
    if (fields.length < 3) {
        console.error("Session format is invalid: " + value);
    }

    return {
        sessionId: fields[0],
        playerId: fields[1],
        username: fields[2],
        country: fields[3]
    };
}