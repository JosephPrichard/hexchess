function getSessionCookie() {
    // only expecting one cookie, take the first one
    const value = ('; ' + document.cookie).split(`; session=`).pop().split(';')[0];
    if (value === "") {
        return undefined;
    }
    return JSON.parse(JSON.parse(value))
}