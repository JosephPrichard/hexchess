package models.entities;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.ToString;

import java.util.List;
import java.util.NoSuchElementException;

@Data
@EqualsAndHashCode
@AllArgsConstructor
public class RankedEntity {
    private long id;
    private int rank;

    public static void joinRanks(List<RankedEntity> rankedList, List<UserEntity> userList) {
        for (UserEntity user : userList) {
            int i = 0;
            for (; i < rankedList.size(); i++) {
                RankedEntity rankedEntity = rankedList.get(i);
                if (rankedEntity.id == user.getId()) {
                    user.setRank(rankedEntity.rank);
                    break;
                }
            }
            if (i >= rankedList.size()) {
                throw new NoSuchElementException();
            }
        }
        userList.sort((e1, e2) -> Float.compare(e1.getRank(), e2.getRank()));
    }
}