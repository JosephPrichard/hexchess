let state = {
    country: undefined
};

async function onSubmitUserForm(e) {
    e.preventDefault();

    const submitElement = document.getElementById("user-form-submit");

    submitElement.innerHTML = '<div class="loader"></div';

    const newUsername = document.getElementById("username").value;
    const newBio = document.getElementById("bio").value;
    const newCountry = state.country;

    const resp = await fetch("/forms/users", {
        method: 'POST',
        headers: getPostHeaders(),
        body: JSON.stringify({
            newUsername,
            newBio,
            newCountry,
        }),
    });
    const code = await resp.text();
    const ok = resp.ok;

    console.log("Update user response", code, ok);

    createNotification(messages[code] || "", ok);
    submitElement.innerHTML = "Login";
}

async function onSelectCountry(country) {
    console.log("Selecting country", country);

    const elemCountry = document.getElementById("selected-country");
    elemCountry.setAttribute('src', `/static/images/flags/${country}.png`);
    elemCountry.setAttribute('alt', country);

    state.country = country;

    toggleCountryDropdown();
}

async function onSubmitPasswordForm(e)  {
    e.preventDefault();

    const submitElem = document.getElementById("password-form-submit");

    submitElem.innerHTML = '<div class="loader"></div>';

    const password = document.getElementById("password").value;
    const newPassword = document.getElementById("new-password").value;
    const confirmNewPassword = document.getElementById("retype-password").value;

    const resp = await fetch("/forms/users/password", {
        method: 'POST',
        headers: getPostHeaders(),
        body: JSON.stringify({
            password,
            newPassword,
            confirmNewPassword,
        }),
    });
    const code = await resp.text();
    const ok = resp.ok;

    console.log("Update password response", code, ok);

    createNotification(messages[code] || "", ok);
    submitElem.innerHTML = "Login";
}

async function onSignOut() {
    const resp = await fetch("/forms/logout", {
        method: 'POST',
        headers: getPostHeaders(),
    });
    const ok = resp.ok;

    if (ok) {
        window.location = "/";
    }
}

function toggleCountryDropdown() {
    const dropdownElement = document.getElementById("country-select-options");

    let display = dropdownElement.style.getPropertyValue("display");
    display = display !== 'none' ? 'none' : 'block';
    dropdownElement.style.setProperty("display", display);
}

function initState(initialCountry) {
    state.country = initialCountry;
}

function attachEventListeners() {
    document.getElementById("update-user-form").addEventListener("submit", onSubmitUserForm);
    document.getElementById("update-password-form").addEventListener("submit", onSubmitPasswordForm);
    document.getElementById("sign-out-button").addEventListener("click", onSignOut);
}