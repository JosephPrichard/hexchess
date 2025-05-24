package chess;

import java.util.Arrays;
import java.util.stream.Stream;

public enum Direction {
    UP,
    DOWN,
    DOWN_LEFT,
    DOWN_RIGHT,
    UP_LEFT,
    UP_RIGHT;

    public final static Direction[][] ROOK_OFFSETS = {
        {Direction.UP},
        {Direction.DOWN},
        {Direction.DOWN_LEFT},
        {Direction.DOWN_RIGHT},
        {Direction.UP_LEFT},
        {Direction.UP_RIGHT}
    };
    public final static Direction[][] BISHOP_OFFSETS = {
        {Direction.UP_RIGHT, Direction.DOWN_RIGHT},
        {Direction.UP_LEFT, Direction.DOWN_LEFT},
        {Direction.UP, Direction.UP_RIGHT},
        {Direction.UP, Direction.UP_LEFT},
        {Direction.DOWN, Direction.DOWN_RIGHT},
        {Direction.DOWN, Direction.DOWN_LEFT}
    };
    public final static Direction[][] KING_OFFSETS =
        Stream.concat(Arrays.stream(ROOK_OFFSETS), Arrays.stream(BISHOP_OFFSETS)).toArray(Direction[][]::new);
    public final static Direction[][] KNIGHT_OFFSETS = {
        {Direction.UP_RIGHT, Direction.UP_RIGHT, Direction.UP},
        {Direction.UP_RIGHT, Direction.UP, Direction.UP},
        {Direction.DOWN_RIGHT, Direction.DOWN_RIGHT, Direction.DOWN},
        {Direction.DOWN_RIGHT, Direction.DOWN, Direction.DOWN},
        {Direction.UP_LEFT, Direction.UP_LEFT, Direction.UP},
        {Direction.UP_LEFT, Direction.UP, Direction.UP},
        {Direction.DOWN_LEFT, Direction.DOWN_LEFT, Direction.DOWN},
        {Direction.DOWN_LEFT, Direction.DOWN, Direction.DOWN},
        {Direction.UP_LEFT, Direction.UP_LEFT, Direction.DOWN_LEFT},
        {Direction.DOWN_LEFT, Direction.DOWN_LEFT, Direction.UP_LEFT},
        {Direction.UP_RIGHT, Direction.UP_RIGHT, Direction.DOWN_RIGHT},
        {Direction.DOWN_RIGHT, Direction.DOWN_RIGHT, Direction.UP_RIGHT},
    };
}