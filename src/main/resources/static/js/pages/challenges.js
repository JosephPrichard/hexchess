async function updateChallenge({ id, buttonId, challengerId, challengeeId, successMsg, failMsg, action }) {
    console.log(`On ${action} challenge`, challengerId, challengeeId);

    const submitElem = document.getElementById(id);
    const buttonElem = document.getElementById(buttonId);

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
    const ok = resp.ok;

    if (ok) {
        submitElem.remove();
        createNotification(successMsg, ok);
    } else {
        createNotification(failMsg, ok);
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
        successMsg: "Deleted challenge against " + challengeeName,
        failMsg: "Failed to deleted challenge against " + challengeeName,
    });
}

async function onAccept(index, challengerId, challengeeId, challengerName) {
    await updateChallenge({
        id: `challenge-${index}`,
        buttonId: `challenge-${index}-accept`,
        challengerId,
        challengeeId,
        action: "Accept",
        successMsg: "Accepted challenge from " + challengerName,
        failMsg: "Failed to accept challenge from " + challengerName,
    });
}

async function onReject(index, challengerId, challengeeId, challengerName) {
    await updateChallenge({
        id: `challenge-${index}`,
        buttonId: `challenge-${index}-reject`,
        challengerId,
        challengeeId,
        action: "Reject",
        successMsg: "Rejected challenge from " + challengerName,
        failMsg: "Failed to reject challenge from " + challengerName,
    });
}