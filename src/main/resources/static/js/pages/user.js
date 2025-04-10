function createGetReplays(userId) {
    let hasMoreRecords = true;
    return async () => {
        const table = document.getElementById('replay-table-tbody');
        const isAtPageBottom = window.innerHeight + window.scrollY >= document.body.offsetHeight;
        if (!hasMoreRecords || !isAtPageBottom || !table.lastElementChild) {
            return;
        }

        const lastId = table.lastElementChild.getAttribute("data-id");

        const resp = await fetch(`/partials/player/replays?userId=${userId}&afterId=${lastId}`, {method: 'GET'});
        const html = await resp.text();
        const ok = resp.ok;

        if (ok) {
            table.insertAdjacentHTML('beforeend', html);
        } else {
            hasMoreRecords = false;
        }
    };
}

async function onCreateChallenge(challengeeId, challengeeName) {
    const resp = await fetch("/forms/challenges/create", {
        method: 'POST',
        headers: {
            "Content-Type": "application/json"
        },
        body: JSON.stringify({ challengeeId })
    });
    const code = await resp.text();
    const ok = resp.ok;

    const customMessages = {
        "ERROR_DUPLICATE_CHALLENGE": "You have already sent a challenge to " + challengeeName,
        "SUCCESS_CREATE_CHALLENGE": "Successfully created a challenge against " + challengeeName
    };
    createNotification(customMessages[code] || messages[code], ok);
}

async function renderUser(userId) {
    const cookie = getSessionCookie();

    const isDifferentUser = cookie && userId !== String(cookie.userId);
    if (isDifferentUser) {
        const elem = document.getElementById("profile-buttons");
        elem.style.removeProperty('display');
    }

    const loadReplays = createGetReplays(userId);
    await loadReplays();
    window.addEventListener('scroll', loadReplays);
}