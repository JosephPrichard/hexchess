function onSubmitLoginForm(e) {
    e.preventDefault();

    var $submitElement = $("#login-form-submit");
    $submitElement.html('<div class="loader"></div>');

    var username = $("#username-login").val();
    var password = $("#password-login").val();

    $.ajax({
        url: "/forms/login",
        type: "POST",
        contentType: "application/json",
        data: JSON.stringify({
            username: username,
            password: password
        }),
        success: function(data) {
            window.location = "/";
            console.log("Login response", data);
        },
        error: function(xhr) {
            var code = xhr.responseText;
            createNotification(message(code), false);
            $submitElement.html("Login");
            console.log("Login response", xhr.responseText);
        }
    });
}

function initLoginPage() {
    $("#login-form").on("submit", onSubmitLoginForm);
}