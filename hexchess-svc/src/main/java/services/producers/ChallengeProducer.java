package services.producers;

import com.fasterxml.jackson.core.JsonProcessingException;
import lombok.AllArgsConstructor;
import services.broadcast.Broadcaster;
import services.broadcast.GlobalBroadcaster;
import utils.Config;
import models.message.ChallengeMsg;

import java.util.Map;

import static utils.Globals.*;

@AllArgsConstructor
public class ChallengeProducer {
    private final Broadcaster userBroadcaster;

    public void broadcastChallenge(ChallengeMsg msg) {
        try {
            String groupId = Long.toString(msg.getChallengeeId());
            LOG.info("Broadcasting challenge={} with groupId={} to user broadcaster", msg, groupId);

            byte[] output = JSON.writeValueAsBytes(msg);
            userBroadcaster.broadcast(groupId, output);
        } catch (JsonProcessingException e) {
            LOG.error("Error occurred while broadcasting challenge to user", e);
        }
    }

    // produces a single message for testing
    public static void main(String[] args) {
        Map<String, String> env = Config.readEnvironment();
        String redisPubsubHost = env.get("REDIS_PUBSUB_HOST");
        int redisPubsubPort = Integer.parseInt(env.get("REDIS_PUBSUB_PORT"));

        ChallengeProducer producer = new ChallengeProducer(new GlobalBroadcaster(Config.getJedisPoolConfig(), redisPubsubHost, redisPubsubPort, Broadcaster.USERS_TOPIC));
        producer.broadcastChallenge(new ChallengeMsg(2, "User2", "us", 1, "User1", "us"));
    }
}
