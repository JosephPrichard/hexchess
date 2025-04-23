async function onSubmitLoginForm(e)  {
    e.preventDefault();

    const submitElement = document.getElementById("login-form-submit");

    submitElement.innerHTML = '<div class="loader"></div>';

    const username = document.getElementById("username-login").value;
    const password = document.getElementById("password-login").value;

    const resp = await fetch("/forms/login", {
        method: 'POST',
        headers: getPostHeaders(),
        body: JSON.stringify({
            username,
            password,
        }),
    });
    const code = await resp.text();
    const ok = resp.ok;

    console.log("Login response", code, ok);

    if (ok) {
        window.location = "/";
    } else {
        createNotification(messages[code] || "", ok);
    }
    submitElement.innerHTML = "Login";
}

function attachEventListeners() {
    document.getElementById("login-form").addEventListener("submit", onSubmitLoginForm);
}