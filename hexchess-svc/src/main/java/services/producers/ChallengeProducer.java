package services.producers;

import com.fasterxml.jackson.core.JsonProcessingException;
import lombok.AllArgsConstructor;
import services.broadcast.Broadcaster;
import services.broadcast.GlobalBroadcaster;
import utils.Config;
import web.dto.ChallengeMsg;

import java.util.Map;

import static utils.Globals.JSON_MAPPER;
import static utils.Globals.LOGGER;

@AllArgsConstructor
public class ChallengeProducer {
    private final Broadcaster userBroadcaster;

    public void broadcastChallenge(ChallengeMsg msg) {
        try {
            String jsonOutput = JSON_MAPPER.writeValueAsString(msg);
            LOGGER.info("Broadcasting challenge={} to user broadcaster", msg);
            userBroadcaster.broadcast(Long.toString(msg.challengeeId), jsonOutput);
        } catch (JsonProcessingException e) {
            LOGGER.error("Error occurred while broadcasting challenge to user", e);
        }
    }

    // produce a single message for testing
    public static void main(String[] args) {
        Map<String, String> env = Config.readEnvironment();
        String redisPubsubHost = env.get("REDIS_PUBSUB_HOST");
        int redisPubsubPort = Integer.parseInt(env.get("REDIS_PUBSUB_PORT"));

        ChallengeProducer producer = new ChallengeProducer(new GlobalBroadcaster(redisPubsubHost, redisPubsubPort, Broadcaster.USERS_TOPIC));
        producer.broadcastChallenge(new ChallengeMsg(2, "User2", "us", 1, "User1", "us"));
    }
}
