async function getHistories(userId, afterId) {
    const url = `/partials/player-history?userId=${userId}&afterId=${afterId}`;
    try {
        const resp = await fetch(url, {method: 'GET'});
        return [await resp.text(), resp.ok];
    } catch (ex) {
        console.error(ex);
        return ["An unexpected error has occurred", false];
    }
}

function createLoadHistories(userId) {
    let hasMoreRecords = true;
    return async () => {
        const table = document.getElementById('history-table-tbody');
        const isAtPageBottom = window.innerHeight + window.scrollY >= document.body.offsetHeight;
        if (!hasMoreRecords || !isAtPageBottom || !table.lastElementChild) {
            return;
        }

        const lastId = table.lastElementChild.getAttribute("data-id");
        const [html, ok] = await getHistories(userId, lastId)
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
    const [text, _] = await createChallenge(challengeeId);
    createNotification(text);
}

function initButtons(userId) {
    const cookie = getSessionCookie();
    if (cookie && userId !== cookie.playerId) {
        const elem = document.getElementById("profile-buttons");
        elem.style.removeProperty('display');
    }
}