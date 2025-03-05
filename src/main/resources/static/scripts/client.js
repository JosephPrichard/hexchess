export async function postLogin(username, password) {
    const formData = new FormData();
    formData.append("username", username);
    formData.append("password", password);

    const url =`/forms/login`;
    try {
        const resp = await fetch(url, { method: 'POST', body: formData });
        return [await resp.text(), resp.ok];
    } catch (ex) {
        console.error(ex);
        return ["An unexpected error has occurred", false];
    }
}

export async function postRegister(username, password, dupPassword) {
    const formData = new FormData();
    formData.append("username", username);
    formData.append("password", password);
    formData.append("duplicate-password", dupPassword);

    const url =`/forms/signup`;
    try {
        const resp = await fetch(url, { method: 'POST', body: formData });
        return [await resp.text(), resp.ok];
    } catch (ex) {
        console.error(ex);
        return ["An unexpected error has occurred", false];
    }
}

export async function postUpdatePassword(password, newPassword, newDupPassword) {
    const formData = new FormData();
    formData.append("password", password);
    formData.append("new-password", newPassword);
    formData.append("duplicate-new-password", newDupPassword);

    const url =`/forms/update-password`;
    try {
        const resp = await fetch(url, { method: 'POST', body: formData });
        return [await resp.text(), resp.ok];
    } catch (ex) {
        console.error(ex);
        return ["An unexpected error has occurred", false];
    }
}

export async function postUpdateUser(username, bio) {
    const formData = new FormData();
    formData.append("new-username", username);
    formData.append("new-bio", bio);

    const url =`/forms/update-user`;
    try {
        const resp = await fetch(url, { method: 'POST', body: formData });
        return [await resp.text(), resp.ok];
    } catch (ex) {
        console.error(ex);
        return ["An unexpected error has occurred", false];
    }
}

export async function postSignOut() {
    const url =`/forms/logout`;
    try {
        const resp = await fetch(url, { method: 'POST' });
        return [await resp.text(), resp.ok];
    } catch (ex) {
        console.error(ex);
        return ["An unexpected error has occurred", false];
    }
}

export async function getUserHistories(userId, afterId) {
    const url =`/partials/player-history?userId=${userId}&afterId=${afterId}`;
    try {
        const resp = await fetch(url, { method: 'GET' });
        return [await resp.text(), resp.ok];
    } catch (ex) {
        console.error(ex);
        return ["", false];
    }
}