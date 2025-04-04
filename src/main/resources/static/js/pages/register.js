async function onSubmitRegisterForm(e) {
    e.preventDefault();

    const usernameElem = document.getElementById("username-register");
    const passwordElem = document.getElementById("password-register");
    const dupPasswordElem = document.getElementById("password-retype-register");
    const submitElem = document.getElementById("register-form-submit");

    submitElem.innerHTML = '<div class="loader"></div>';

    const resp = await fetch("/forms/register", {
        method: 'POST',
        headers: {
            "Content-Type": "application/json"
        },
        body: JSON.stringify({
            username: usernameElem.value,
            password: passwordElem.value,
            confirmPassword: dupPasswordElem.value
        })
    });
    const text = await resp.text();
    const ok = resp.ok;

    console.log("Register response", text, ok);

    if (ok) {
        window.location = "/";
    } else {
        createNotification(text, ok);
    }
    submitElem.innerHTML = "Register";
}

const registerFormElem = document.getElementById("register-form");
registerFormElem.addEventListener('submit', onSubmitRegisterForm);