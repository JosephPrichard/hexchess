async function postLogin(username, password) {
    const formData = new FormData();
    formData.append("username", username);
    formData.append("password", password);

    const url = `/forms/login`;
    try {
        const resp = await fetch(url, {method: 'POST', body: formData});
        return [await resp.text(), resp.ok];
    } catch (ex) {
        console.error(ex);
        return ["An unexpected error has occurred", false];
    }
}

const loginForm = document.getElementById("login-form");

loginForm.addEventListener("submit", async (e) => {
    e.preventDefault();

    const usernameElem = document.getElementById("username-login");
    const passwordElem = document.getElementById("password-login");
    const submitElem = document.getElementById("login-form-submit");

    submitElem.innerHTML = '<div class="loader"></div>';

    const username = usernameElem.value;
    const password = passwordElem.value;

    const [text, ok] = await postLogin(username, password);
    console.log("Login response", text, ok);

    const messageElem = document.getElementById("login-form-message");

    if (ok) {
        messageElem.innerText = "";
        messageElem.className = "";
        window.location = "/";
    } else {
        messageElem.innerText = text;
        messageElem.className = "red-color";
    }

    submitElem.innerHTML = "Login";
});