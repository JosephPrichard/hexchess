var country = "";

function onSubmitUserForm(e) {
    e.preventDefault();

    var $submitElement = $("#user-form-submit");
    $submitElement.html('<div class="loader"></div>');

    var newUsername = $("#username").val();
    var newBio = $("#bio").val();

    $.ajax({
        url: "/forms/users",
        type: "POST",
        contentType: "application/json",
        data: JSON.stringify({
            newUsername: newUsername,
            newBio: newBio,
            newCountry: country
        }),
        success: function(data) {
            createNotification(message(data), true);
            $submitElement.html("Login");
            console.log("User Form response", data);
        },
        error: function(xhr) {
            var code = xhr.responseText;
            createNotification(message(code), false);
            $submitElement.html("Login");
            console.log("User Form response", xhr.responseText);
        }
    });
}

function onSelectCountry(newCountry) {
    var $elemCountry = $("#selected-country");
    $elemCountry.attr("src", "/static/images/flags/" + newCountry + ".png");
    $elemCountry.attr("alt", newCountry);

    country = newCountry;

    toggleCountryDropdown();
}

function onSubmitPasswordForm(e) {
    e.preventDefault();

    var $submitElem = $("#password-form-submit");
    $submitElem.html('<div class="loader"></div>');

    var password = $("#password").val();
    var newPassword = $("#new-password").val();
    var confirmNewPassword = $("#retype-password").val();

    $.ajax({
        url: "/forms/users/password",
        type: "POST",
        contentType: "application/json",
        data: JSON.stringify({
            password: password,
            newPassword: newPassword,
            confirmNewPassword: confirmNewPassword
        }),
        success: function(data) {
            createNotification(message(data), true);
            $submitElem.html("Login");
            console.log("Update password response", data, true);
        },
        error: function(xhr) {
            var code = xhr.responseText;
            createNotification(message(code), false);
            $submitElem.html("Login");
            console.log("Update password response", code, false);
        }
    });
}

function onSignOut() {
    $.ajax({
        url: "/forms/logout",
        type: "POST",
        contentType: "application/json",
        success: function() {
            window.location = "/";
        }
    });
}

function toggleCountryDropdown() {
    var $dropdownElement = $("#country-select-options");
    var display = $dropdownElement.css("display");
    $dropdownElement.css("display", display !== "none" ? "none" : "block");
}

function initProfilePage(initialCountry) {
    country = initialCountry;

    $("#update-user-form").on("submit", onSubmitUserForm);
    $("#update-password-form").on("submit", onSubmitPasswordForm);
    $("#sign-out-button").on("click", onSignOut);
}
