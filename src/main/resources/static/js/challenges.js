function updateChallenge(args) {
    var id = args.id;
    var buttonId = args.buttonId;
    var challengerId = Number(args.challengerId);
    var challengeeId = Number(args.challengeeId);
    var action = args.action;
    var messageOfResp = args.messageOfResp;

    var $buttonElement = $("#" + buttonId);

    console.log("On " + action + " challenge", challengerId, challengeeId);
    $buttonElement.html('<div class="loader"></div>');

    $.ajax({
        url: "/forms/challenges/update",
        type: "POST",
        contentType: "application/json",
        data: JSON.stringify({
            challengerId: challengerId,
            challengeeId: challengeeId,
            action: action
        }),
        success: function (data) {
            var msg = messageOfResp(data);
            createNotification(msg, true);

            var $noneElement = $("#no-challenges");
            var $listElement = $("#challenge-list");
            var $submitElement = $("#" + id);

            $submitElement.remove();

            if ($listElement.children().length === 0) {
                $noneElement.css("display", "block");
                $listElement.remove();
            }

            console.log($listElement.children().length + " challenges after removal");
            $buttonElement.html(action);
        },
        error: function (xhr) {
            var respBody = JSON.parse(xhr.responseText);
            var msg = getMessage(respBody);
            createNotification(msg, false);
            $buttonElement.html(action);
        }
    });
}

function onDelete(index, challengerId, challengeeId, challengeeName) {
    updateChallenge({
        id: "challenge-" + index,
        buttonId: "challenge-" + index + "-delete",
        challengerId: challengerId,
        challengeeId: challengeeId,
        action: "Delete",
        messageOfResp: function (resp) {
            if (resp.code === "SUCCESS_UPDATE_CHALLENGE") {
                return "Deleted challenge against " + challengeeName;
            }
            return message(resp.code);
        }
    });
}

function onAccept(index, challengerId, challengeeId, challengerName) {
    updateChallenge({
        id: "challenge-" + index,
        buttonId: "challenge-" + index + "-accept",
        challengerId: challengerId,
        challengeeId: challengeeId,
        action: "Accept",
        messageOfResp: function (resp) {
            if (resp.code === "SUCCESS_UPDATE_CHALLENGE") {
                return "Accepted challenge from " + challengerName;
            }
            return message(resp.code);
        }
    });
}

function onReject(index, challengerId, challengeeId, challengerName) {
    updateChallenge({
        id: "challenge-" + index,
        buttonId: "challenge-" + index + "-reject",
        challengerId: challengerId,
        challengeeId: challengeeId,
        action: "Reject",
        messageOfResp: function (resp) {
            if (resp.code === "SUCCESS_UPDATE_CHALLENGE") {
                return "Rejected challenge from " + challengerName;
            }
            return message(resp.code);
        }
    });
}