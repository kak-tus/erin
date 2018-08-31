package parser

import (
	"sync"

	"git.aqq.me/go/nanachi"
	"github.com/peterbourgon/diskv"
	"go.uber.org/zap"
)

// Parser object
type Parser struct {
	logger  *zap.SugaredLogger
	C       chan string
	m       *sync.Mutex
	toMove  map[string]bool
	config  parserConfig
	diskv   *diskv.Diskv
	nanachi *nanachi.Client
	dest    *nanachi.Destination
}

type parserConfig struct {
	MovePath  string
	StorePath string
	QueueName string
	MaxShard  int32
}
