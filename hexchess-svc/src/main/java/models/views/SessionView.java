package models.views;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;

import services.daos.UserDao.*;

@NoArgsConstructor
@AllArgsConstructor
@Data
public class SessionView {
    public long id;
    public String username;
    public String country;
    public float elo;
    @EqualsAndHashCode.Exclude
    public Long ttlSecs;

    public static SessionView fromUser(VerifiedUser user, Long maxAge) {
        return new SessionView(user.id, user.username, user.country, user.elo, maxAge);
    }
}
