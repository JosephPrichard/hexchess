async function updateChallenge(challengerId, challengeeId, action) {
    const url = "/forms/update-challenge";
    try {
        const formData = new FormData();
        formData.set("challengerId", challengerId);
        formData.set("challengeeId", challengeeId);
        formData.set("action", action);

        const resp = await fetch(url, {method: 'POST', body: formData });
        return [await resp.text(), resp.ok];
    } catch (ex) {
        console.error(ex);
        return ["", false];
    }
}

async function onDelete(index, challengerId, challengeeId) {
    const id = `challenge-${index}`;
    const buttonId = `challenge-${index}-delete`;
    console.log("On delete challenge ", challengerId, challengeeId);

    const submitElem = document.getElementById(id);
    const buttonElem = document.getElementById(buttonId);

    buttonElem.innerHTML = '<div class="loader"></div>';

    const [text, ok] = await updateChallenge(challengerId, challengeeId, "DELETE");
    if (ok) {
        submitElem.remove();
    }

    buttonElem.innerHTML = "Delete";
}

async function onAccept(index, challengerId, challengeeId) {
    const id = `challenge-${index}`;
    const buttonId = `challenge-${index}-accept`;
    console.log("On accept challenge ", challengerId, challengeeId);

    const submitElem = document.getElementById(id);
    const buttonElem = document.getElementById(buttonId);

    buttonElem.innerHTML = '<div class="loader"></div>';

    const [text, ok] = await updateChallenge(challengerId, challengeeId, "ACCEPT");
    if (ok) {
        submitElem.remove();
    }

    buttonElem.innerHTML = "Accept";
}

async function onReject(index, challengerId, challengeeId) {
    const id = `challenge-${index}`;
    const buttonId = `challenge-${index}-reject`;
    console.log("On reject challenge ", challengerId, challengeeId);

    const submitElem = document.getElementById(id);
    const buttonElem = document.getElementById(buttonId);

    buttonElem.innerHTML = '<div class="loader"></div>';

    const [text, ok] = await updateChallenge(challengerId, challengeeId, "REJECT");
    if (ok) {
        submitElem.remove();
    }

    buttonElem.innerHTML = "Reject";
}