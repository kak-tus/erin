package founder

import (
	"path/filepath"
	"regexp"
	"sync"

	"git.aqq.me/go/app/appconf"
	"git.aqq.me/go/app/applog"
	"git.aqq.me/go/app/event"
	"github.com/fsnotify/fsnotify"
	"github.com/kak-tus/erin/parser"
)

var fnd *Founder

func init() {
	event.Init.AddHandler(
		func() error {
			cnfMap := appconf.GetConfig()["founder"]

			var cnf founderConfig
			err := appconf.Decode(cnfMap, &cnf)
			if err != nil {
				return err
			}

			wtch, err := fsnotify.NewWatcher()
			if err != nil {
				return err
			}

			fnd = &Founder{
				logger:  applog.GetLogger().Sugar(),
				config:  cnf,
				watcher: wtch,
				parser:  parser.GetParser(),
				regexp:  regexp.MustCompile(cnf.Regexp),
				m:       &sync.Mutex{},
			}

			fnd.logger.Info("Started founder")

			return nil
		},
	)

	event.Stop.AddHandler(
		func() error {
			fnd.logger.Info("Stop founder")
			fnd.watcher.Close()
			fnd.m.Lock()

			fnd.parser.Stop()
			fnd.logger.Info("Stopped founder")
			return nil
		},
	)
}

// GetFounder return instance
func GetFounder() *Founder {
	return fnd
}

// Start find files
func (f *Founder) Start() {
	go f.parser.Start()

	f.m.Lock()

	f.findInitial()

	err := f.watcher.Add(f.config.DumpPath)
	if err != nil {
		f.logger.Panic(err)
	}

LOOP:
	for {
		select {
		case ev, more := <-f.watcher.Events:
			if !more {
				break LOOP
			}

			if !(ev.Op == fsnotify.Create || ev.Op == fsnotify.Write) {
				continue
			}

			matchPattern, err := filepath.Match(filepath.Join(f.config.DumpPath, f.config.Pattern), ev.Name)
			if err != nil {
				f.logger.Error(err)
			}

			if matchPattern && f.matchRegexp(ev.Name) {
				f.parser.C <- ev.Name
			}
		case err, more := <-f.watcher.Errors:
			if !more {
				break LOOP
			}

			f.logger.Error(err)
		}
	}

	f.m.Unlock()
}

func (f *Founder) findInitial() {
	list, err := filepath.Glob(filepath.Join(f.config.DumpPath, f.config.Pattern))
	if err != nil {
		f.logger.Error(err)
		return
	}

	for _, file := range list {
		if f.matchRegexp(file) {
			f.parser.C <- file
		}
	}
}

func (f *Founder) matchRegexp(file string) bool {
	// connectionName_connectionID_20060102_150405.pcap
	_, name := filepath.Split(file)

	if len(name) < 24 {
		return false
	}

	return f.regexp.MatchString(name)
}
