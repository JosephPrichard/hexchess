let state = {
    country: undefined
};

async function onSubmitUserForm(e) {
    e.preventDefault();

    const usernameElem = document.getElementById("username");
    const bioElem = document.getElementById("bio");
    const submitElem = document.getElementById("user-form-submit");

    submitElem.innerHTML = '<div class="loader"></div';

    const resp = await fetch("/forms/users", {
        method: 'POST',
        headers: getPostHeaders(),
        body: JSON.stringify({
            newUsername: usernameElem.value,
            newBio: bioElem.value,
            newCountry: state.country,
        }),
    });
    const code = await resp.text();
    const ok = resp.ok;

    console.log("Update user response", code, ok);

    createNotification(messages[code] || "", ok);
    submitElem.innerHTML = "Login";
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

    const passwordElem = document.getElementById("password");
    const newPasswordElem = document.getElementById("new-password");
    const retypePasswordElem = document.getElementById("retype-password");
    const submitElem = document.getElementById("password-form-submit");

    submitElem.innerHTML = '<div class="loader"></div>';

    const resp = await fetch("/forms/users/password", {
        method: 'POST',
        headers: getPostHeaders(),
        body: JSON.stringify({
            password: passwordElem.value,
            newPassword: newPasswordElem.value,
            confirmNewPassword: retypePasswordElem.value,
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
    const dropdown = document.getElementById("country-select-options");

    let display = dropdown.style.getPropertyValue("display");
    display = display !== 'none' ? 'none' : 'block';
    dropdown.style.setProperty("display", display);
}

function initState(initialCountry) {
    state.country = initialCountry;
}

function attachEventListeners() {
    const userFormElem = document.getElementById("update-user-form");
    const passwordFormElem = document.getElementById("update-password-form");
    const signOutButton = document.getElementById("sign-out-button");

    userFormElem.addEventListener("submit", onSubmitUserForm);
    passwordFormElem.addEventListener("submit", onSubmitPasswordForm);
    signOutButton.addEventListener("click", onSignOut);
}