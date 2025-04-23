async function onSubmitRegisterForm(e) {
    e.preventDefault();

    const submitElement = document.getElementById("register-form-submit");

    submitElement.innerHTML = '<div class="loader"></div>';

    const username = document.getElementById("username-register").value;
    const password = document.getElementById("password-register").value;
    const confirmPassword = document.getElementById("password-retype-register").value;

    const resp = await fetch("/forms/register", {
        method: 'POST',
        headers: getPostHeaders(),
        body: JSON.stringify({
            username,
            password,
            confirmPassword,
        }),
    });
    const code = await resp.text();
    const ok = resp.ok;

    if (ok) {
        window.location = "/";
    } else {
        createNotification(messages[code] || "", ok);
    }
    submitElement.innerHTML = "Register";
}

function attachEventListeners() {
    document.getElementById("register-form").addEventListener('submit', onSubmitRegisterForm);
}