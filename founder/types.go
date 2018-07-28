package founder

import (
	"regexp"

	"github.com/fsnotify/fsnotify"
	"github.com/kak-tus/erin/parser"
	"go.uber.org/zap"
)

type founderConfig struct {
	DumpPath string
	Pattern  string
	Regexp   string
}

// Founder hold object
type Founder struct {
	logger  *zap.SugaredLogger
	config  founderConfig
	watcher *fsnotify.Watcher
	parser  *parser.Parser
	regexp  *regexp.Regexp
}
