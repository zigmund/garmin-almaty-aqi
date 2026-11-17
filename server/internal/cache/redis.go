package cache

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// unlockScript deletes the lock only if it still holds the caller's token.
var unlockScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
	return redis.call('DEL', KEYS[1])
end
return 0
`)

// takeScript implements the sliding-window budget atomically.
// KEYS[1] budget zset; ARGV: now (ms), window (ms), limit, member.
// A take exactly one window old leaves the window. Returns 1 on success, 0 if
// the limit is reached.
var takeScript = redis.NewScript(`
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now - window)
if redis.call('ZCARD', KEYS[1]) < tonumber(ARGV[3]) then
	redis.call('ZADD', KEYS[1], now, ARGV[4])
	redis.call('PEXPIRE', KEYS[1], window)
	return 1
end
return 0
`)

// Redis is a Store and Budget backed by a Redis instance shared between
// server replicas. Redis errors are always returned unchanged.
type Redis struct {
	rdb     *redis.Client
	prefix  string
	perHour int
	now     func() time.Time
}

// NewRedis returns a Redis-backed cache. All keys start with prefix. now is
// the clock used for the budget window (not the Redis server time).
func NewRedis(rdb *redis.Client, prefix string, perHour int, now func() time.Time) *Redis {
	return &Redis{rdb: rdb, prefix: prefix, perHour: perHour, now: now}
}

func (r *Redis) entryKey(key string) string { return r.prefix + "v1:" + key }
func (r *Redis) lockKey(name string) string { return r.prefix + "lock:" + name }
func (r *Redis) budgetKey() string          { return r.prefix + "budget" }

func randomToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// Get implements Store.
func (r *Redis) Get(ctx context.Context, key string) (Entry, bool, error) {
	raw, err := r.rdb.Get(ctx, r.entryKey(key)).Bytes()
	if errors.Is(err, redis.Nil) {
		return Entry{}, false, nil
	}
	if err != nil {
		return Entry{}, false, err
	}
	var e Entry
	if err := json.Unmarshal(raw, &e); err != nil {
		return Entry{}, false, err
	}
	return e, true, nil
}

// Set implements Store.
func (r *Redis) Set(ctx context.Context, key string, e Entry, ttl time.Duration) error {
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return r.rdb.Set(ctx, r.entryKey(key), raw, ttl).Err()
}

// Lock implements Store.
func (r *Redis) Lock(ctx context.Context, name string, ttl time.Duration) (func(), bool, error) {
	tok, err := randomToken()
	if err != nil {
		return nil, false, err
	}
	k := r.lockKey(name)
	ok, err := r.rdb.SetNX(ctx, k, tok, ttl).Result()
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	return func() {
		// Best effort and independent of the caller's context, which may
		// already be cancelled; the lock expires by TTL anyway.
		uctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = unlockScript.Run(uctx, r.rdb, []string{k}, tok).Err()
	}, true, nil
}

// Take implements Budget.
func (r *Redis) Take(ctx context.Context) error {
	member, err := randomToken()
	if err != nil {
		return err
	}
	res, err := takeScript.Run(ctx, r.rdb, []string{r.budgetKey()},
		r.now().UnixMilli(), budgetWindow.Milliseconds(), strconv.Itoa(r.perHour), member).Int()
	if err != nil {
		return err
	}
	if res != 1 {
		return ErrBudget
	}
	return nil
}
