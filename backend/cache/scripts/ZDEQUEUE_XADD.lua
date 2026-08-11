-- @ARGS
-- KEYS[1] zSetName string - the sorted set name to dequeue elements from
-- KEYS[2] streamName string - the stream name to publish
-- ARGV[1] beforeRank string - dequeue all elements up until this rank
-- @RETURNS the elements that were dequeued

local function zDequeueXAdd(zSetName, streamName, beforeRank)
    local due = redis.call('ZRANGEBYSCORE', zSetName, '-inf', beforeRank)

    if #due == 0 then
        -- No elements to poll, so return nothing
        return {}
    end

    -- Delete the retrieved elements from the sorted set
    redis.call('ZREM', zSetName, unpack(due))

    -- Publish each element into the stream (an object that contains an "id" field is a standard that my queues that receive ids use)
    for index, value in ipairs(due) do
        local payload = cjson.encode({ id = value })
        redis.call('XADD', streamName, '*', 'data', payload)
    end

    return due
end

return zDequeueXAdd(KEYS[1], KEYS[2], ARGV[1])