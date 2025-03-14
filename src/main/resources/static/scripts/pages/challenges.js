async function updateChallenge(challengerId, challengeeId, action) {
    const url = "/forms/update-challenge";
    try {
        const form = new FormData();
        form.set("challengerId", challengerId);
        form.set("challengeeId", challengeeId);
        form.set("action", action);

        const resp = await fetch(url, {method: 'POST'});
        return [await resp.text(), resp.ok];
    } catch (ex) {
        console.error(ex);
        return ["", false];
    }
}

function onDelete(id, challengerId, challengeeId) {
    console.log("On delete challenge ", challengerId, challengeeId);

    updateChallenge(challengerId, challengeeId, "DELETE")
        .then(([text, ok]) => {
            if (ok) {
                const element = document.getElementById(id);
                element.remove();
            }
        });
}

function onAccept(challengerId, challengeeId) {
    console.log("On accept challenge ", challengerId, challengeeId);

    updateChallenge(challengerId, challengeeId, "ACCEPT")
        .then(([text, ok]) => {
            if (ok) {
                const element = document.getElementById(id);
                element.remove();
            }
        });
}

function onReject(id, challengerId, challengeeId) {
    console.log("On reject challenge ", challengerId, challengeeId);

    updateChallenge(challengerId, challengeeId, "REJECT")
        .then(([text, ok]) => {
            if (ok) {
                const statusElem = document.getElementById(id);
                statusElem.innerHTML = "Rejected";
                statusElem.className = "red-color";
            }
        });
}