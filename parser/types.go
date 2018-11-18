package parser

import (
	"sync"
	"time"

	"git.aqq.me/go/retrier"
	"github.com/go-redis/redis"
	"github.com/kak-tus/ami"
	"go.uber.org/zap"
)

// Parser object
type Parser struct {
	logger   *zap.SugaredLogger
	C        chan string
	m        *sync.Mutex
	toMove   map[string]bool
	config   parserConfig
	location *time.Location
	redisdb  *redis.ClusterClient
	retrier  *retrier.Retrier
	pr       *ami.Producer
}

type parserConfig struct {
	MovePath          string
	QueueName         string
	Redis             redisConfig
	ShardsCount       int8
	PendingBufferSize int64
	PipeBufferSize    int64
}

type redisConfig struct {
	Addrs    string
	Password string
}

const (
	redisTTL = time.Hour * 24
)
