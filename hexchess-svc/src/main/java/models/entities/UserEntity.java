package models.entities;

import lombok.*;

import java.sql.Timestamp;

@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class UserEntity {
    public static final float START_ELO = 1000f;

    private long id;
    private String username;
    private String country;
    private float elo;
    private float highestElo;
    private int wins;
    private int losses;
    private int rank;
    private String bio;
    @EqualsAndHashCode.Exclude
    private Timestamp joinedOn;

    public UserEntity(long id, String username, String country) {
        this(id, username, country, 0, 0, 0, 0, 0, null, null);
    }

    public UserEntity(long id, String username, String country, float elo, int rank) {
        this(id, username, country, elo, elo, 0, 0, rank, null, null);
    }
}