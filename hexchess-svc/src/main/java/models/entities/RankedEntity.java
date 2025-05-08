package models.entities;

import lombok.AllArgsConstructor;
import lombok.EqualsAndHashCode;
import lombok.ToString;

import java.util.List;
import java.util.NoSuchElementException;

@ToString
@EqualsAndHashCode
@AllArgsConstructor
public class RankedEntity {
    public long id;
    public int rank;

    public static void joinRanks(List<RankedEntity> rankedList, List<UserEntity> userList) {
        for (UserEntity user : userList) {
            int i = 0;
            for (; i < rankedList.size(); i++) {
                RankedEntity rankedEntity = rankedList.get(i);
                if (rankedEntity.id == user.id) {
                    user.rank = rankedEntity.rank;
                    break;
                }
            }
            if (i >= rankedList.size()) {
                throw new NoSuchElementException();
            }
        }
        userList.sort((e1, e2) -> Float.compare(e1.rank, e2.rank));
    }
}