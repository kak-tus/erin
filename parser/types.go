package parser

import (
	"sync"
	"time"

	"git.aqq.me/go/nanachi"
	"git.aqq.me/go/retrier"
	"github.com/go-redis/redis"
	"github.com/peterbourgon/diskv"
	"go.uber.org/zap"
)

// Parser object
type Parser struct {
	logger   *zap.SugaredLogger
	C        chan string
	m        *sync.Mutex
	toMove   map[string]bool
	config   parserConfig
	diskv    *diskv.Diskv
	nanachi  *nanachi.Client
	producer *nanachi.SmartProducer
	location *time.Location
	redisdb  *redis.ClusterClient
	retrier  *retrier.Retrier
}

type parserConfig struct {
	MovePath  string
	StorePath string
	QueueName string
	MaxShard  int32
	URI       string
	Redis     redisConfig
}

type redisConfig struct {
	Addrs    string
	Password string
}

const (
	redisTTL = time.Hour * 24
)
