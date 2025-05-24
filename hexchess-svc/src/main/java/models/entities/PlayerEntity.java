package models.entities;

import lombok.*;

@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class PlayerEntity {
    private long id;
    @EqualsAndHashCode.Exclude
    private String name;
    @EqualsAndHashCode.Exclude
    private String country;
    @EqualsAndHashCode.Exclude
    private Float elo;
    private boolean isGuest = false;

    public PlayerEntity(long id, String name, String country, Float elo) {
        this.id = id;
        this.name = name;
        this.country = country;
        this.elo = elo;
    }
}