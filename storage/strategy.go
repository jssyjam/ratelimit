package storage

import "time"

const (
	// 默认限流速率
	DefaultInterval = 50 * time.Millisecond
	// 默认令牌桶大小
	DefaultBucket = 100
)

// Strategy 限流策略
type Strategy struct {
	// ratelimit strategy name
	Name string `json:"name"`
	// 令牌桶大小
	Bucket int `json:"bucket"`
	// 限流开关
	Off bool `json:"off"`

	Interval time.Duration
}
