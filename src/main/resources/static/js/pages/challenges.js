async function updateChallenge({ id, buttonId, challengerId, challengeeId, action, updateMessages }) {
    console.log(`On ${action} challenge`, challengerId, challengeeId);

    const submitElem = document.getElementById(id);
    const buttonElem = document.getElementById(buttonId);
    const noChallengesElem = document.getElementById("no-challenges");
    const challengeListElem = document.getElementById("challenge-list");

    buttonElem.innerHTML = '<div class="loader"></div>';

    const resp = await fetch("/forms/challenges/update", {
        method: 'POST',
        headers: {
            "Content-Type": "application/json"
        },
        body: JSON.stringify({
            challengerId: Number(challengerId),
            challengeeId: Number(challengeeId),
            action
        })
    });
    const code = await resp.text();
    const ok = resp.ok;

    const msg = updateMessages[code] || messages[code]
    createNotification(msg, ok);

    if (ok) {
        submitElem.remove();
        if (challengeListElem.children.length === 0) {
            noChallengesElem.style.setProperty('display', 'block');
            challengeListElem.remove();
        }
        console.log(`${challengeListElem.children.length} challenges after removal`)
    }
    buttonElem.innerHTML = action;
}

async function onDelete(index, challengerId, challengeeId, challengeeName) {
    await updateChallenge({
        id: `challenge-${index}`,
        buttonId: `challenge-${index}-delete`,
        challengerId,
        challengeeId,
        action: "Delete",
        updateMessages: {
            "SUCCESS_UPDATE_CHALLENGE": "Deleted challenge against " + challengeeName
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
        updateMessages: {
            "SUCCESS_UPDATE_CHALLENGE": "Accepted challenge from " + challengerName,
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
        updateMessages: {
            "SUCCESS_UPDATE_CHALLENGE": "Rejected challenge from " + challengerName
        }
    });
}