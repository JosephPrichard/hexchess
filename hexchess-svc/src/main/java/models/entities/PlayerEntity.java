package models.entities;

import lombok.*;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class PlayerEntity {
    public long id;
    @EqualsAndHashCode.Exclude
    public String name;
    @EqualsAndHashCode.Exclude
    public String country;
    @EqualsAndHashCode.Exclude
    public Float elo;
    public boolean isGuest = false;

    public PlayerEntity(long id, String name, String country, Float elo) {
        this.id = id;
        this.name = name;
        this.country = country;
        this.elo = elo;
    }
}