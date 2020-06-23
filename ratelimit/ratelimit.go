package ratelimit

import (
	"fmt"
	"log"
	"math"
	"time"

	"github.com/jssyjam/ratelimit/storage"

	"github.com/go-redis/redis"
)

// Limit defines the maximum frequency of some events.
// Limit is represented as number of events per second.
// A zero Limit allows no events.
type Limit float64

type Limiter struct {
	key        string
	interval   time.Duration
	bucket     int
	off        bool
	client     *redis.Client
	scriptHash string
}

const (
	Inf = Limit(math.MaxFloat64)
)

// NewLimiter returns a new Limiter that allows events up to rate r and permits
// brusts of at most b tokens.
func NewLimiter(client *redis.Client, strategy *storage.Strategy, hash string) *Limiter {
	return &Limiter{
		client:     client,
		key:        strategy.Name,
		interval:   strategy.Interval,
		bucket:     strategy.Bucket,
		off:        strategy.Off,
		scriptHash: hash,
	}
}

// Every converts a minimum time interval between events to a Limit.
func Every(interval time.Duration) Limit {
	if interval <= 0 {
		return Inf
	}
	return 1 / Limit(interval.Seconds())
}

// Allow is shorthand for AllowN(time.Now(), 1).
func (lim *Limiter) Allow() Reservation {
	return lim.AllowN(time.Now(), 1)
}

// AllowN reports whether n events may happen at time now.
// Use this method if you intend to drop / skip events that exceed the rate limit.
// Otherwise use Reserve or Wait.
func (lim *Limiter) AllowN(now time.Time, n int) Reservation {
	return lim.reserveN(now, n)
}

// Off 检测限流是否已经关闭
func (lim *Limiter) Off() bool {
	return lim.off
}

// Brust 令牌桶容量
func (lim *Limiter) Brust() int {
	return lim.bucket
}

// Key 令牌标识
func (lim *Limiter) Key() string {
	return lim.key
}

// QPS 平均 qps
func (lim *Limiter) QPS() float64 {
	qps := time.Second / lim.interval
	return math.Ceil(float64(qps))
}

// A Reservation holds information about events that are permitted by a Limiter to happen after a delay.
// A Reservation may be canceled, which may enable the Limiter to permit additional events.
type Reservation struct {
	OK     bool
	Tokens int
}

func (lim *Limiter) reserveN(now time.Time, n int) Reservation {
	if lim.client == nil {
		return Reservation{
			OK:     true,
			Tokens: n,
		}
	}
	results, err := lim.client.EvalSha(
		lim.scriptHash,
		[]string{genTokenKey(lim.key), genTimeKey(lim.key)},
		float64(Every(lim.interval)),
		lim.bucket,
		now.Unix(),
		n,
	).Result()
	if err != nil {
		log.Println("fail to call rate limit: ", err)
		return Reservation{
			OK:     true,
			Tokens: n,
		}
	}
	rs, ok := results.([]interface{})
	if ok {
		newTokens, _ := rs[1].(int64)
		return Reservation{
			OK:     rs[0] == int64(1),
			Tokens: int(newTokens),
		}
	}

	log.Println("fail to transform results")
	return Reservation{
		OK:     true,
		Tokens: n,
	}
}

const script = `
local tokens_key = KEYS[1]
local timestamp_key = KEYS[2]

local rate = tonumber(ARGV[1])
local capacity = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])

local fill_time = capacity/rate
local ttl = math.floor(fill_time*2)

local last_tokens = tonumber(redis.call("get", tokens_key))
if last_tokens == nil then
    last_tokens = capacity
end

local last_refreshed = tonumber(redis.call("get", timestamp_key))
if last_refreshed == nil then
    last_refreshed = 0
end

local delta = math.max(0, now-last_refreshed)
local filled_tokens = math.min(capacity, last_tokens+(delta*rate))
local allowed = filled_tokens >= requested
local new_tokens = filled_tokens
if allowed then
    new_tokens = filled_tokens - requested
end

redis.call("setex", tokens_key, ttl, new_tokens)
redis.call("setex", timestamp_key, ttl, now)

return { allowed, new_tokens }
`

func genTokenKey(key string) string {
	return fmt.Sprintf("{%s}.token", key)
}

func genTimeKey(key string) string {
	return fmt.Sprintf("{%s}.ts", key)
}
