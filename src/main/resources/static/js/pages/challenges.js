async function updateChallenge({ id, buttonId, challengerId, challengeeId, action, getMessage }) {
    const buttonElement = document.getElementById(buttonId);

    console.log(`On ${action} challenge`, challengerId, challengeeId);

    buttonElement.innerHTML = '<div class="loader"></div>';

    challengerId = Number(challengerId);
    challengeeId = Number(challengeeId);

    const resp = await fetch("/forms/challenges/update", {
        method: 'POST',
        headers: getPostHeaders(),
        body: JSON.stringify({
            challengerId,
            challengeeId,
            action,
        })
    });
    const object = await resp.json();
    const ok = resp.ok;

    const msg = getMessage(object);
    createNotification(msg, ok);

    if (ok) {
        const noneElement = document.getElementById("no-challenges");
        const listElement = document.getElementById("challenge-list");
        const submitElement = document.getElementById(id);

        submitElement.remove();
        if (listElement.children.length === 0) {
            noneElement.style.setProperty('display', 'block');
            listElement.remove();
        }

        console.log(`${listElement.children.length} challenges after removal`)
    }
    buttonElement.innerHTML = action;
}

async function onDelete(index, challengerId, challengeeId, challengeeName) {
    await updateChallenge({
        id: `challenge-${index}`,
        buttonId: `challenge-${index}-delete`,
        challengerId,
        challengeeId,
        action: "Delete",
        getMessage: (object) => {
            if (object.code === "SUCCESS_UPDATE_CHALLENGE") {
                return "Deleted challenge against " + challengeeName;
            }
            return messages[code] || "";
        }
    });
}

async function onAccept(index, challengerId, challengeeId, challengerName) {
    await updateChallenge({
        id: `challenge-${index}`,
        buttonId: `challenge-${index}-accept`,
        challengerId,
        challengeeId,
        action: "Accept",
        getMessage: (object) => {
            if (object.code === "SUCCESS_UPDATE_CHALLENGE") {
                return "Accepted challenge from " + challengerName;
            }
            return messages[code] || "";
        }
    });
}

async function onReject(index, challengerId, challengeeId, challengerName) {
    await updateChallenge({
        id: `challenge-${index}`,
        buttonId: `challenge-${index}-reject`,
        challengerId,
        challengeeId,
        action: "Reject",
        getMessage: (object) => {
            if (object.code === "SUCCESS_UPDATE_CHALLENGE") {
                return "Rejected challenge from " + challengerName;
            }
            return messages[code] || "";
        }
    });
}