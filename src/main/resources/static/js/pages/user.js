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

async function onCreateChallenge(challengeeId) {
    const resp = await fetch("/forms/challenges/create", {
        method: 'POST',
        headers: {
            "Content-Type": "application/json"
        },
        body: JSON.stringify({ challengeeId })
    });
    const text = await resp.text();
    const ok = resp.ok;

    createNotification(text, ok);
}

function initButtons(userId) {
    const cookie = getSessionCookie();

    const isDifferentUser = cookie && userId !== String(cookie.userId);
    console.log("Initialize profile buttons", cookie.userId, userId, userId !== cookie.userId);

    if (isDifferentUser) {
        const elem = document.getElementById("profile-buttons");
        elem.style.removeProperty('display');
    }
}