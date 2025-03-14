async function postRegister(username, password, dupPassword) {
    const formData = new FormData();
    formData.append("username", username);
    formData.append("password", password);
    formData.append("duplicate-password", dupPassword);

    const url = `/forms/register`;
    try {
        const resp = await fetch(url, {method: 'POST', body: formData});
        return [await resp.text(), resp.ok];
    } catch (ex) {
        console.error(ex);
        return ["An unexpected error has occurred", false];
    }
}

const signupForm = document.getElementById("register-form");

signupForm.addEventListener('submit', async (e) => {
    e.preventDefault();

    const usernameElem = document.getElementById("username-register");
    const passwordElem = document.getElementById("password-register");
    const dupPasswordElem = document.getElementById("password-retype-register");
    const submitElem = document.getElementById("register-form-submit");

    submitElem.innerHTML = '<div class="loader"></div>';

    const username = usernameElem.value;
    const password = passwordElem.value;
    const dupPassword = dupPasswordElem.value;

    const [text, ok] = await postRegister(username, password, dupPassword);
    console.log("Register response", text, ok);

    const messageElem = document.getElementById("register-form-message");

    if (ok) {
        messageElem.innerText = "";
        messageElem.className = "";
        window.location = "/";
    } else {
        messageElem.innerText = text;
        messageElem.className = "red-color";
    }

    submitElem.innerHTML = "Register";
});