async function getPlayerReplays(userId, afterId) {
    const url = `/partials/player/replays?userId=${userId}&afterId=${afterId}`;
    try {
        const resp = await fetch(url, {method: 'GET'});
        return [await resp.text(), resp.ok];
    } catch (ex) {
        console.error(ex);
        return ["An unexpected error has occurred", false];
    }
}

function createLoadReplays(userId) {
    let hasMoreRecords = true;
    return async () => {
        const table = document.getElementById('replay-table-tbody');
        const isAtPageBottom = window.innerHeight + window.scrollY >= document.body.offsetHeight;
        if (!hasMoreRecords || !isAtPageBottom || !table.lastElementChild) {
            return;
        }

        const lastId = table.lastElementChild.getAttribute("data-id");
        const [html, ok] = await getPlayerReplays(userId, lastId)
        if (ok) {
            table.insertAdjacentHTML('beforeend', html);
        } else {
            hasMoreRecords = false;
        }
    };
}

async function createChallenge(challengeeId) {
    const url = "/forms/create-challenge";
    try {
        const formData = new FormData();
        formData.set("challengeeId", challengeeId);

        const resp = await fetch(url, {method: 'POST', body: formData });
        return [await resp.text(), resp.ok];
    } catch (ex) {
        console.error(ex);
        return ["", false];
    }
}

async function onCreateChallenge(challengeeId) {
    const [text, ok] = await createChallenge(challengeeId);
    createNotification(text, ok);
}

function initButtons(userId) {
    const cookie = getSessionCookie();
    if (cookie && userId !== cookie.userId) {
        const elem = document.getElementById("profile-buttons");
        elem.style.removeProperty('display');
    }
}