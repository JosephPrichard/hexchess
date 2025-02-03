package models;

import lombok.AllArgsConstructor;
import lombok.Data;

import java.util.List;
import java.util.NoSuchElementException;

@Data
@AllArgsConstructor
public class RankedUser {
    public String id;
    public int rank;

    public static void joinRanks(List<RankedUser> rankedList, List<UserEntity> entityList) {
        for (var entity : entityList) {
            int i = 0;
            for (; i < rankedList.size(); i++) {
                var rankedUser = rankedList.get(i);
                if (rankedUser.id.equals(entity.id)) {
                    entity.rank = rankedUser.rank;
                    break;
                }
            }
            if (i >= rankedList.size()) {
                throw new NoSuchElementException();
            }
        }
        entityList.sort((e1, e2) -> Float.compare(e1.rank, e2.rank));
    }
}