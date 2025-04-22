async function onSubmitRegisterForm(e) {
    e.preventDefault();

    const usernameElem = document.getElementById("username-register");
    const passwordElem = document.getElementById("password-register");
    const dupPasswordElem = document.getElementById("password-retype-register");
    const submitElem = document.getElementById("register-form-submit");

    submitElem.innerHTML = '<div class="loader"></div>';

    const resp = await fetch("/forms/register", {
        method: 'POST',
        headers: getPostHeaders(),
        body: JSON.stringify({
            username: usernameElem.value,
            password: passwordElem.value,
            confirmPassword: dupPasswordElem.value,
        }),
    });
    const code = await resp.text();
    const ok = resp.ok;

    if (ok) {
        window.location = "/";
    } else {
        createNotification(messages[code] || "", ok);
    }
    submitElem.innerHTML = "Register";
}

function attachEventListeners() {
    const registerFormElem = document.getElementById("register-form");
    registerFormElem.addEventListener('submit', onSubmitRegisterForm);
}