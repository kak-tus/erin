package founder

import (
	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

type founderConfig struct {
	DumpPath string
}

// Founder hold object
type Founder struct {
	logger  *zap.SugaredLogger
	config  founderConfig
	watcher *fsnotify.Watcher
}
