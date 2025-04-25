$(document).ready(function() {
    var cookieStr = ('; ' + document.cookie).split('; session=').pop().split(';')[0];
    var cookie = cookieStr !== "" ? JSON.parse(JSON.parse(cookieStr)) : undefined;

    if (cookie) {
        var $elem = $("#user-link");
        $elem.css("display", "");
        $elem.html(cookie.username);
        $elem.attr("href", "/players/" + cookie.userId);
    } else {
        $("#login-link").css("display", "");
        $("#challenge-link").css("display", "none");
        $("#settings-link").css("display", "none");
    }
})