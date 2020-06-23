package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"github.com/jssyjam/ratelimit"
	"github.com/jssyjam/ratelimit/storage"
)

type Keys struct {
	Header []string
}

type GinMiddleware struct {
	Buckets *ratelimit.Buckets
	keys    *Keys
}

func NewGinMiddleware(client *redis.Client, storage storage.Storage) (*GinMiddleware, error) {
	buckets, err := ratelimit.NewBuckets(client, storage)
	if err != nil {
		return nil, err
	}
	return &GinMiddleware{Buckets: buckets, keys: &Keys{}}, nil
}

func (gm *GinMiddleware) RegisterBucketKey(keys []string) {
	gm.keys = &Keys{Header: keys}
}

func (gm *GinMiddleware) RateLimit() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		key := gm.genBucketKey(ctx)
		limiter, err := gm.Buckets.Get(key)
		if err != nil {
			return
		}
		if limiter.Off() {
			return
		}
		if limiter.Allow().OK {
			return
		}
		ctx.AbortWithError(http.StatusTooManyRequests, errors.New("too many requests"))
	}
}

func (gm *GinMiddleware) genBucketKey(ctx *gin.Context) string {
	keys := []string{ctx.Request.URL.Path}
	for _, header := range gm.keys.Header {
		val := ctx.Request.Header.Get(header)
		if val != "" {
			keys = append(keys, val)
		}
	}
	return strings.Join(keys, "/")
}
