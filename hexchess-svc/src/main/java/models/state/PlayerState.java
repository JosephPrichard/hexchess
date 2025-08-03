package models.state;

import com.google.protobuf.InvalidProtocolBufferException;
import lombok.*;
import messages.Messages;

import static utils.Globals.LOG;

@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class PlayerState {
    private long id;
    @EqualsAndHashCode.Exclude
    @NonNull
    private String name = "";
    @EqualsAndHashCode.Exclude
    @NonNull
    private String country = "";
    @EqualsAndHashCode.Exclude
    private float elo;
    @EqualsAndHashCode.Exclude
    private boolean isGuest = false;

    public PlayerState(long id) {
        this.id = id;
    }

    public PlayerState(long id, @NonNull String name, @NonNull String country, float elo) {
        this.id = id;
        this.name = name;
        this.country = country;
        this.elo = elo;
    }

    public Messages.Player serialize() {
        return Messages.Player.newBuilder()
            .setId(id)
            .setName(name)
            .setCountry(country)
            .setElo(0.0f)
            .setIsGuest(isGuest)
            .build();
    }

    public static PlayerState deserialize(Messages.Player msg) {
        return new PlayerState(msg.getId(), msg.getName(), msg.getCountry(), msg.getElo());
    }

    public byte[] serializeAsBytes() {
        return serialize().toByteArray();
    }

    public static PlayerState deserialize(byte[] bytes) {
        try {
            return deserialize(Messages.Player.parseFrom(bytes));
        } catch (InvalidProtocolBufferException ex) {
            LOG.error("Failed to deserialize player object from bytes", ex);
            throw new RuntimeException(ex);
        }
    }
}