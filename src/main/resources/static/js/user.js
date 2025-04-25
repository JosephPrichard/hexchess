function createGetReplays(userId) {
    var hasMoreRecords = true;
    return function () {
        var $tableElement = $('#replay-table-tbody');

        var isAtPageBottom = window.innerHeight + window.scrollY >= document.body.offsetHeight;
        if (!hasMoreRecords || !isAtPageBottom || !$tableElement.children().last().length) {
            return;
        }

        var lastId = $tableElement.children().last().attr("data-id");

        $.ajax({
            url: "/partials/player/replays",
            type: "GET",
            data: {
                userId: userId,
                afterId: lastId
            },
            success: function (html) {
                $tableElement.append(html);
            },
            error: function () {
                hasMoreRecords = false;
            }
        });
    };
}

function onCreateChallenge(challengeeId, challengeeName) {
    var customMessages = {
        "ERROR_DUPLICATE_CHALLENGE": "You have already sent a challenge to " + challengeeName,
        "SUCCESS_CREATE_CHALLENGE": "Successfully created a challenge against " + challengeeName
    };
    $.ajax({
        url: "/forms/challenges/create",
        type: "POST",
        contentType: "application/json",
        data: JSON.stringify({
            challengeeId: challengeeId
        }),
        success: function (data) {
            createNotification(customMessages[data] || message(data), true);
        },
        error: function (xhr) {
            var code = xhr.responseText;
            createNotification(customMessages[code] || message(code), false);
        }
    });
}

function initUserPage(userId) {
    var cookieStr = ('; ' + document.cookie).split('; session=').pop().split(';')[0];
    var cookie = cookieStr !== "" ? JSON.parse(JSON.parse(cookieStr)) : undefined;

    var isDifferentUser = cookie && userId !== String(cookie.userId);
    if (isDifferentUser) {
        $("#profile-buttons").css("display", "");

        var loadReplays = createGetReplays(userId);
        loadReplays();
        $(window).on("scroll", loadReplays);
    }
}