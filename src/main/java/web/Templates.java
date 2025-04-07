package web;

import com.github.jknack.handlebars.Handlebars;
import com.github.jknack.handlebars.Template;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.io.IOException;

@Data
@NoArgsConstructor
public class Templates {

    private Template indexTemplate;
    private Template loginTemplate;
    private Template registerTemplate;
    private Template leaderboardTemplate;
    private Template userTemplate;
    private Template profileTemplate;
    private Template currentGamesTemplate;
    private Template replayTemplate;
    private Template searchTemplate;
    private Template challengesTemplate;
    private Template errorTemplate;

    private Template replayListTemplate;

    public Templates(Handlebars handlebars) throws IOException {
        // pages
        indexTemplate = handlebars.compile("/pages/index");
        registerTemplate = handlebars.compile("/pages/register");
        loginTemplate = handlebars.compile("/pages/login");
        leaderboardTemplate = handlebars.compile("/pages/leaderboard");
        userTemplate = handlebars.compile("/pages/user");
        profileTemplate = handlebars.compile("/pages/profile");
        currentGamesTemplate = handlebars.compile("/pages/currentGames");
        replayTemplate = handlebars.compile("/pages/replay");
        searchTemplate = handlebars.compile("/pages/search");
        challengesTemplate = handlebars.compile("/pages/challenges");
        errorTemplate = handlebars.compile("/pages/error");

        // partials
        replayListTemplate = handlebars.compile("partials/replayList");
    }
}
