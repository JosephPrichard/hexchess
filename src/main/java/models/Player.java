package models;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class Player {
    public String id;
    @EqualsAndHashCode.Exclude
    public String name;

    public Player deepCopy() {
        return new Player(id, name);
    }
}