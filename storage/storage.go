package storage

import "context"

const (
	EventTypeDel = "delete"
	EventTypePut = "put"
)

type Event struct {
	EventType string
	Strategy  *Strategy
}

// Storage 限流策略存储
type Storage interface {
	Save(ctx context.Context, s *Strategy) error
	Watch(ctx context.Context) (chan *Event, error)
	// Get(ctx context.Context) (*Strategy, error)
	List(ctx context.Context) ([]*Strategy, error)
}

// DefaultStorage 默认策略存储
type DefaultStorage struct {
	limiters map[string]*Strategy
}

func NewDefaultStorage() Storage {
	return &DefaultStorage{
		limiters: make(map[string]*Strategy, 0),
	}
}

func (ds *DefaultStorage) Save(ctx context.Context, s *Strategy) error {
	ds.limiters[s.Name] = s
	return nil
}

func (ds *DefaultStorage) Watch(ctx context.Context) (chan *Event, error) {
	return nil, nil
}

func (ds *DefaultStorage) List(ctx context.Context) ([]*Strategy, error) {
	strategys := make([]*Strategy, len(ds.limiters))
	for _, strategy := range ds.limiters {
		strategys = append(strategys, strategy)
	}
	return nil, nil
}
