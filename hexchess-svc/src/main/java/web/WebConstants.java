package web;

public class WebConstants {
    // http error codes
    public static final String ERROR_UNKNOWN = "ERROR_UNKNOWN";
    public static final String ERROR_INVALID_PASSWORD = "ERROR_PASSWORD_LENGTH";
    public static final String ERROR_CONFIRM_PASSWORD = "ERROR_CONFIRM_PASSWORD";
    public static final String ERROR_INVALID_USERNAME = "ERROR_USERNAME_LENGTH";
    public static final String ERROR_UNSAFE_USERNAME = "ERROR_UNSAFE_USERNAME";
    public static final String ERROR_INVALID_PARTICIPANTS = "ERROR_INVALID_PARTICIPANTS";
    public static final String ERROR_DUPLICATE_USERNAME = "ERROR_DUPLICATE_USERNAME";
    public static final String ERROR_INVALID_LOGIN = "ERROR_INVALID_LOGIN";
    public static final String ERROR_REQUIRED_LOGIN = "ERROR_REQUIRED_LOGIN";
    public static final String ERROR_SESSION_EXPIRED = "ERROR_SESSION_EXPIRED";
    public static final String ERROR_NOT_FOUND_CHALLENGE = "ERROR_NOT_FOUND_CHALLENGE";
    public static final String ERROR_INVALID_CHALLENGE_ACTION = "ERROR_INVALID_CHALLENGE_ACTION";
    public static final String ERROR_SELF_CHALLENGE = "ERROR_SELF_CHALLENGE";
    public static final String ERROR_DUPLICATE_CHALLENGE = "ERROR_DUPLICATE_CHALLENGE";
    public static final String ERROR_UPDATE_CHALLENGE = "ERROR_UPDATE_CHALLENGE";
    public static final String ERROR_INVALID_REQUEST = "ERROR_INVALID_REQUEST";

    // http success codes
    public static final String SUCCESS_LOGIN = "SUCCESS_LOGIN";
    public static final String SUCCESS_REGISTER = "SUCCESS_REGISTER";
    public static final String SUCCESS_UPDATE_PASSWORD = "SUCCESS_UPDATE_PASSWORD";
    public static final String SUCCESS_UPDATE_USER = "SUCCESS_UPDATE_USER";
    public static final String SUCCESS_CREATE_CHALLENGE = "SUCCESS_CREATE_CHALLENGE";
    public static final String SUCCESS_UPDATE_CHALLENGE = "SUCCESS_UPDATE_CHALLENGE";
    public static final String SUCCESS_GENERIC = "SUCCESS";

    // ws response codes
    public static final String ERROR_MESSAGE_TYPE = "ERROR_MESSAGE_TYPE";
    public static final String ERROR_TURN = "ERROR_TURN";
    public static final String ERROR_INVALID_MOVE = "ERROR_INVALID_MOVE";
    public static final String ERROR_FINISHED_GAME = "ERROR_FINISHED_GAME";
    public static final String ERROR_INVALID_GAME = "ERROR_INVALID_GAME";
}
