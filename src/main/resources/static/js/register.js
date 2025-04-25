function onSubmitRegisterForm(e) {
    e.preventDefault();

    var $submitElement = $("#register-form-submit");
    $submitElement.html('<div class="loader"></div>');

    var username = $("#username-register").val();
    var password = $("#password-register").val();
    var confirmPassword = $("#password-retype-register").val();

    $.ajax({
        url: "/forms/register",
        type: "POST",
        contentType: "application/json",
        data: JSON.stringify({
            username: username,
            password: password,
            confirmPassword: confirmPassword
        }),
        success: function(data) {
            window.location = "/";
            console.log("Register response", data);
        },
        error: function(xhr) {
            var code = xhr.responseText;
            createNotification(message(code), false);
            $submitElement.html("Register");
            console.log("Register response", xhr.responseText);
        }
    });
}

function initRegisterPage() {
    $("#register-form").on("submit", onSubmitRegisterForm);
}