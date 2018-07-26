package parser

import (
	"sync"

	"go.uber.org/zap"
)

// Parser object
type Parser struct {
	logger *zap.SugaredLogger
	C      chan string
	m      *sync.Mutex
}
