package models;

import lombok.*;

import java.sql.Timestamp;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class UserEntity {
    public static final float START_ELO = 1000f;

    public long id;
    public String username;
    public String country;
    public float elo;
    public float highestElo;
    public int wins;
    public int losses;
    public int rank;
    public String bio;
    @EqualsAndHashCode.Exclude
    public Timestamp joinedOn;

    public UserEntity(long id, String username, String country) {
        this(id, username, country, 0, 0, 0, 0, 0, null, null);
    }

    public UserEntity(long id, String username, String country, float elo, int rank) {
        this(id, username, country, elo, elo, 0, 0, rank, null, null);
    }

    public void roundElo() {
        elo = Math.round(elo);
        highestElo = Math.round(highestElo);
    }
}