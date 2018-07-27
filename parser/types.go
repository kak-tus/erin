package parser

import (
	"sync"

	"github.com/peterbourgon/diskv"
	"go.uber.org/zap"
)

// Parser object
type Parser struct {
	logger *zap.SugaredLogger
	C      chan string
	m      *sync.Mutex
	toMove map[string]bool
	config parserConfig
	diskv  *diskv.Diskv
}

type parserConfig struct {
	MovePath  string
	StorePath string
}
