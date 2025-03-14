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

const updateUserForm = document.getElementById("update-user-form");
const updatePasswordForm = document.getElementById("update-password-form");
const signOutButton = document.getElementById("sign-out-button");

updateUserForm.addEventListener("submit", async (e) => {
    e.preventDefault();

    const usernameElem = document.getElementById("username");
    const bioElem = document.getElementById("bio");
    const submitElem = document.getElementById("user-form-submit");

    submitElem.innerHTML = `<div class="loader"></div`;

    const [text, ok] = await postUpdateUser(usernameElem.value, bioElem.value);
    console.log("Update user response", text, ok);

    const messageElem = document.getElementById("update-user-form-message");
    messageElem.innerText = text;

    if (ok) {
        messageElem.className = "";
    } else {
        messageElem.className = "red-color";
    }
    submitElem.innerHTML = "Login";
})

updatePasswordForm.addEventListener("submit", async (e) => {
    e.preventDefault();

    const passwordElem = document.getElementById("password");
    const newPasswordElem = document.getElementById("new-password");
    const retypePasswordElem = document.getElementById("retype-password");
    const submitElem = document.getElementById("password-form-submit");

    submitElem.innerHTML = '<div class="loader"></div>';

    const [text, ok] = await postUpdatePassword(passwordElem.value, newPasswordElem.value, retypePasswordElem.value);
    console.log("Update password response", text, ok);

    const messageElem = document.getElementById("update-password-form-message");
    messageElem.innerText = text;

    if (ok) {
        messageElem.className = "";
    } else {
        messageElem.className = "red-color";
    }
    submitElem.innerHTML = "Login";
})

signOutButton.addEventListener('click', async (_) => {
    const [object, ok] = await postSignOut();
    console.log("Logout response", object, ok);
    window.location = "/";
});
