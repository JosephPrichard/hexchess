async function onSubmitLoginForm(e)  {
    e.preventDefault();

    const usernameElem = document.getElementById("username-login");
    const passwordElem = document.getElementById("password-login");
    const submitElem = document.getElementById("login-form-submit");

    submitElem.innerHTML = '<div class="loader"></div>';

    const resp = await fetch("/forms/login", {
        method: 'POST',
        headers: {
          "Content-Type": "application/json"
        },
        body: JSON.stringify({
            username: usernameElem.value,
            password: passwordElem.value
        })
    });
    const text = await resp.text();
    const ok = resp.ok;

    console.log("Login response", text, ok);

    if (ok) {
        window.location = "/";
    } else {
        createNotification(text, ok);
    }
    submitElem.innerHTML = "Login";
}

const loginFormElem = document.getElementById("login-form");
loginFormElem.addEventListener("submit", onSubmitLoginForm);