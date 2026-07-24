-- KEYS[1] = bucket key
-- ARGV    = now_ms, capacity, window_ms
-- returns   1 if admitted, 0 if rejected

local now_ms    = tonumber(ARGV[1])
local capacity  = tonumber(ARGV[2])
local window_ms = tonumber(ARGV[3])

local data  = redis.call('HMGET', KEYS[1], 'level', 'ts')
local level = tonumber(data[1])
local ts    = tonumber(data[2])
if level == nil then level = 0 end
if ts == nil then ts = now_ms end

local elapsed = now_ms - ts
if elapsed < 0 then elapsed = 0 end

-- leak at capacity/window per ms; multiply before dividing to keep it exact.
level = level - (elapsed * capacity) / window_ms
if level < 0 then level = 0 end

local allowed = 0
if level + 1 <= capacity then
  level = level + 1
  allowed = 1
end

redis.call('HSET', KEYS[1], 'level', level, 'ts', now_ms)
redis.call('PEXPIRE', KEYS[1], window_ms)

return allowed
