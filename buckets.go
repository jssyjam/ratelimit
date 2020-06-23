package ratelimit

import (
	"context"
	"crypto/sha1"
	"fmt"
	"time"

	"github.com/jssyjam/ratelimit/storage"

	"github.com/go-redis/redis"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	checkScriptInterval = 10 * time.Second
)

type Buckets struct {
	scriptHash string
	client     *redis.Client
	limiters   map[string]*Limiter
	storage    storage.Storage
}

func NewBuckets(client *redis.Client, store storage.Storage) (*Buckets, error) {
	manager := &Buckets{
		client: client,
	}
	if err := manager.loadScript(); err != nil {
		return nil, err
	}
	if store == nil {
		// use default strategy storage
		manager.storage = storage.NewDefaultStorage()
	} else {
		manager.storage = store
	}
	// load all ratelimit strategy
	strats, err := manager.storage.List(context.Background())
	if err != nil {
		return nil, err
	}

	for _, strat := range strats {
		limiter := NewLimiter(client, strat, manager.scriptHash)
		manager.limiters[strat.Name] = limiter
	}
	// start the gorouting to keep script alive
	manager.Backend(context.Background())
	return manager, nil
}

func (b *Buckets) Backend(ctx context.Context) {
	go b.watchEvent(ctx)
	go b.keepScriptAlive(ctx)
}

func (b *Buckets) keepScriptAlive(ctx context.Context) error {
	timer := time.NewTicker(checkScriptInterval)
	for {
		select {
		case <-timer.C:
			if err := b.loadScript(); err != nil {
				log.Errorf("error to load script:%s", err.Error())
			}
		case <-ctx.Done():
			log.Infof("keep script alive operation has been cancel")
			return ctx.Err()
		}
	}
}

func (b *Buckets) watchEvent(ctx context.Context) error {
	events, err := b.storage.Watch(ctx)
	if err != nil {
		return err
	}

	for {
		select {
		case e := <-events:
			if err := b.parseEvent(e); err != nil {
				log.Errorf("error to parse the event:%+v err:%s", e, err)
			}
		case <-ctx.Done():
			log.Warnf("watch operation has been cancel")
			return ctx.Err()
		}
	}
}

func (b *Buckets) Get(key string) (*Limiter, error) {
	var err error
	ctx := context.Background()
	limiter, ok := b.limiters[key]
	if ok {
		return limiter, nil
	}
	strat := &storage.Strategy{
		Name:     key,
		Interval: storage.DefaultInterval,
		Bucket:   storage.DefaultBucket,
	}
	limiter = NewLimiter(b.client, strat, b.scriptHash)
	if err = b.storage.Save(ctx, strat); err != nil {
		return nil, errors.Wrapf(err, "save the record:%+v", *strat)
	}
	log.Infof("save strategy:%s successfully", strat.Name)
	return limiter, nil
}

func (b *Buckets) parseEvent(event *storage.Event) error {
	switch event.EventType {
	case storage.EventTypePut:
		limiter := NewLimiter(b.client, event.Strategy, b.scriptHash)
		b.limiters[event.Strategy.Name] = limiter
		// delete the key in the redis
		if err := b.client.Del(genTokenKey(limiter.key)).Err(); err != nil {
			return errors.Wrapf(err, "error to delete the token key:%s", limiter.key)
		}
		if err := b.client.Del(genTimeKey(limiter.key)).Err(); err != nil {
			return errors.Wrapf(err, "error to delete the time key:%s", limiter.key)
		}
	case storage.EventTypeDel:
		// TODO: pass
		return nil
	default:
		return errors.Errorf("not support the event type:%s", event.EventType)
	}
	return nil
}

func (b *Buckets) loadScript() error {
	if b.client == nil {
		return errors.New("redis client is nil")
	}

	b.scriptHash = fmt.Sprintf("%x", sha1.Sum([]byte(script)))
	exists, err := b.client.ScriptExists(b.scriptHash).Result()
	if err != nil {
		return err
	}

	// load script when missing.
	if !exists[0] {
		_, err := b.client.ScriptLoad(script).Result()
		if err != nil {
			return err
		}
	}
	return nil
}
