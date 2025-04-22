package models;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.ToString;

import java.util.List;
import java.util.NoSuchElementException;

@ToString
@EqualsAndHashCode
@AllArgsConstructor
public class RankedUser {
    public long id;
    public int rank;

    public static void joinRanks(List<RankedUser> rankedList, List<UserEntity> userList) {
        for (UserEntity user : userList) {
            int i = 0;
            for (; i < rankedList.size(); i++) {
                RankedUser rankedUser = rankedList.get(i);
                if (rankedUser.id == user.id) {
                    user.rank = rankedUser.rank;
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