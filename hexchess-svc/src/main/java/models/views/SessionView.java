package models.views;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;

import models.state.PlayerState;
import services.UserDao.*;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class SessionView {
    private long id;
    private String username = "";
    private String country = "";
    private float elo;
    @EqualsAndHashCode.Exclude
    private Long ttlSecs;

    public static SessionView fromUser(VerifiedUser user, Long maxAge) {
        return new SessionView(user.getId(), user.getUsername(), user.getCountry(), user.getElo(), maxAge);
    }

    public static SessionView fromPlayer(PlayerState player, Long maxAge) {
        return new SessionView(player.getId(), player.getName(), player.getCountry(), player.getElo(), maxAge);
    }
}
