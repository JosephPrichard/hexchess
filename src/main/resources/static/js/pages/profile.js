async function postUpdatePassword(password, newPassword, newDupPassword) {
    const formData = new FormData();
    formData.append("password", password);
    formData.append("new-password", newPassword);
    formData.append("duplicate-new-password", newDupPassword);

    const url = `/forms/update-password`;
    try {
        const resp = await fetch(url, {method: 'POST', body: formData});
        return [await resp.text(), resp.ok];
    } catch (ex) {
        console.error(ex);
        return ["An unexpected error has occurred", false];
    }
}

async function postUpdateUser(username, bio) {
    const formData = new FormData();
    formData.append("new-username", username);
    formData.append("new-bio", bio);

    const url = `/forms/update-user`;
    try {
        const resp = await fetch(url, {method: 'POST', body: formData});
        return [await resp.text(), resp.ok];
    } catch (ex) {
        console.error(ex);
        return ["An unexpected error has occurred", false];
    }
}

async function postSignOut() {
    const url = `/forms/logout`;
    try {
        const resp = await fetch(url, {method: 'POST'});
        return [await resp.text(), resp.ok];
    } catch (ex) {
        console.error(ex);
        return ["An unexpected error has occurred", false];
    }
}

async function postUpdateCountry(country) {
    const formData = new FormData();
    formData.append("new-country", country);

    const url = `/forms/update-user`;
    try {
        const resp = await fetch(url, {method: 'POST', body: formData});
        return [await resp.text(), resp.ok];
    } catch (ex) {
        console.error(ex);
        return ["An unexpected error has occurred", false];
    }
}

async function onSubmitUserForm(e) {
    e.preventDefault();

    const usernameElem = document.getElementById("username");
    const bioElem = document.getElementById("bio");
    const submitElem = document.getElementById("user-form-submit");

    submitElem.innerHTML = `<div class="loader"></div`;

    const [text, ok] = await postUpdateUser(usernameElem.value, bioElem.value);
    console.log("Update user response", text, ok);

    createNotification(text, ok);
    submitElem.innerHTML = "Login";
}

async function onSubmitPasswordForm(e)  {
    e.preventDefault();

    const passwordElem = document.getElementById("password");
    const newPasswordElem = document.getElementById("new-password");
    const retypePasswordElem = document.getElementById("retype-password");
    const submitElem = document.getElementById("password-form-submit");

    submitElem.innerHTML = '<div class="loader"></div>';

    const [text, ok] = await postUpdatePassword(passwordElem.value, newPasswordElem.value, retypePasswordElem.value);
    console.log("Update password response", text, ok);

    createNotification(text, ok);
    submitElem.innerHTML = "Login";
}

async function onSignOut(_) {
    const [object, ok] = await postSignOut();
    console.log("Logout response", object, ok);
    window.location = "/";
}

function toggleCountryDropdown() {
    const dropdown = document.getElementById("country-select-options");
    let display = dropdown.style.getPropertyValue("display");
    display = display !== 'none' ? 'none' : 'block';
    dropdown.style.setProperty("display", display);
}

async function onSelectCountry(country) {
    console.log("Selecting country", country);

    const [text, ok] = await postUpdateCountry(country);

    if (ok) {
        const elemCountry = document.getElementById("selected-country");
        elemCountry.src = `/static/images/flags/${country}.png`;
    }
    createNotification(text, ok);
}

document.getElementById("update-user-form").addEventListener("submit", onSubmitUserForm);
document.getElementById("update-password-form").addEventListener("submit", onSubmitPasswordForm);
document.getElementById("sign-out-button").addEventListener("click", onSignOut);