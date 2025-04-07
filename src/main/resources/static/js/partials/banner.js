window.addEventListener('load', () => {
    const cookie = getSessionCookie();

    if (cookie) {
        const elem = document.getElementById("user-link");
        elem.style.removeProperty('display');
        elem.innerHTML = cookie.username;
        elem.setAttribute("href", `/players/${cookie.userId}`);
    } else {
        let elem = document.getElementById("login-link");
        elem.style.removeProperty('display');

        elem = document.getElementById("challenge-link");
        elem.style.setProperty('display', 'none');

        elem = document.getElementById("settings-link");
        elem.style.setProperty('display', 'none');
    }
});