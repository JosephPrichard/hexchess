function updateChallenge(args) {
    var id = args.id;
    var buttonId = args.buttonId;
    var challengerId = Number(args.challengerId);
    var challengeeId = Number(args.challengeeId);
    var buttonText = args.buttonText;
    var messageOfResp = args.messageOfResp;

    var $button = $("#" + buttonId);

    console.log("On " + buttonText + " challenge", challengerId, challengeeId);
    $button.html('<div class="loader"></div>');

    $.ajax({
        url: "/forms/challenges/update",
        type: "POST",
        contentType: "application/json",
        data: JSON.stringify({
            challengerId: challengerId,
            challengeeId: challengeeId,
            action: buttonText
        }),
        success: function (data) {
            var msg = messageOfResp(data);
            createNotification(msg, true);

            var $none = $("#no-challenges");
            var $challengeList = $("#challenge-list");
            var $submit = $("#" + id);

            $submit.remove();

            if ($challengeList.children().length === 0) {
                $none.css("display", "block");
                $challengeList.remove();
            }

            $button.html(buttonText);

            console.log($challengeList.children().length + " challenges after removal");
        },
        error: function (xhr) {
            var respBody = JSON.parse(xhr.responseText);
            var msg = messageOfResp(respBody);
            createNotification(msg, false);
            $button.html(buttonText);

            console.log("Update challenge response", xhr.responseText);
        }
    });
}

function onDelete(index, challengerId, challengeeId, challengeeName) {
    updateChallenge({
        id: "challenge-" + index,
        buttonId: "challenge-" + index + "-delete",
        challengerId: challengerId,
        challengeeId: challengeeId,
        buttonText: "Delete",
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
        buttonText: "Accept",
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
        buttonText: "Reject",
        messageOfResp: function (resp) {
            if (resp.code === "SUCCESS_UPDATE_CHALLENGE") {
                return "Rejected challenge from " + challengerName;
            }
            return message(resp.code);
        }
    });
}