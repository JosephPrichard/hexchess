package models.entities;

import lombok.*;

import java.sql.Timestamp;

@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class ReplayEntity {
    public static final int WHITE_WIN = 0;
    public static final int BLACK_WIN = 1;
    public static final int DRAW = 2;
    public static final int CHECKMATE = 0;
    public static final int FORFEIT = 1;

    private long id;
    private long whiteId;
    private long blackId;
    private String whiteName = "";
    private String blackName = "";
    private String whiteCountry = "";
    private String blackCountry = "";
    private int result = DRAW;
    private int cause = CHECKMATE;
    private float winElo;
    private float loseElo;
    private float whiteElo;
    private float blackElo;
    @EqualsAndHashCode.Exclude
    private Timestamp playedOn;
}