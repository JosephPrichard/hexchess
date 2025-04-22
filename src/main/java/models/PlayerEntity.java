package models;

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
}