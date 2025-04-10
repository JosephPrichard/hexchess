package models;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;

import java.sql.Timestamp;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class ReplayEntity {
    public static final int WHITE_WIN = 0;
    public static final int BLACK_WIN = 1;
    public static final int DRAW = 2;

    public long id;
    public long whiteId;
    public long blackId;
    public String whiteName;
    public String blackName;
    public String whiteCountry;
    public String blackCountry;
    public int result;
    public float winElo;
    public float loseElo;
    public float whiteElo;
    public float blackElo;
    public String moveListJson;
    @EqualsAndHashCode.Exclude
    public Timestamp playedOn;
}