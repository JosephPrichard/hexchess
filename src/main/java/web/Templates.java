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
    private Template profileTemplate;
    private Template preferencesTemplates;
    private Template currentGamesTemplate;
    private Template replayTemplate;
    private Template searchTemplate;
    private Template challengesTemplate;
    private Template errorTemplate;

    private Template historyListTemplate;

    public Templates(Handlebars handlebars) throws IOException {
        // pages
        indexTemplate = handlebars.compile("/pages/index");
        registerTemplate = handlebars.compile("/pages/register");
        loginTemplate = handlebars.compile("/pages/login");
        leaderboardTemplate = handlebars.compile("/pages/leaderboard");
        profileTemplate = handlebars.compile("/pages/profile");
        preferencesTemplates = handlebars.compile("/pages/preferences");
        currentGamesTemplate = handlebars.compile("/pages/currentGames");
        replayTemplate = handlebars.compile("/pages/replay");
        searchTemplate = handlebars.compile("/pages/searchPlayers");
        challengesTemplate = handlebars.compile("/pages/challenges");
        errorTemplate = handlebars.compile("/pages/error");

        // partials
        historyListTemplate = handlebars.compile("partials/historyList");
    }
}
