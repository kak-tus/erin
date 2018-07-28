package founder

import (
	"path/filepath"
	"regexp"

	"git.aqq.me/go/app/appconf"
	"git.aqq.me/go/app/applog"
	"git.aqq.me/go/app/event"
	"github.com/fsnotify/fsnotify"
	"github.com/kak-tus/erin/parser"
	"github.com/mitchellh/mapstructure"
)

var fnd *Founder

func init() {
	event.Init.AddHandler(
		func() error {
			cnfMap := appconf.GetConfig()["founder"]

			var cnf founderConfig
			err := mapstructure.Decode(cnfMap, &cnf)
			if err != nil {
				return err
			}

			wtch, err := fsnotify.NewWatcher()
			if err != nil {
				return err
			}

			fnd = &Founder{
				logger:  applog.GetLogger(),
				config:  cnf,
				watcher: wtch,
				parser:  parser.GetParser(),
				regexp:  regexp.MustCompile(cnf.Regexp),
			}

			fnd.logger.Info("Started founder")

			return nil
		},
	)

	event.Stop.AddHandler(
		func() error {
			fnd.logger.Info("Stop founder")
			fnd.watcher.Close()
			return nil
		},
	)
}

// GetFounder return instance
func GetFounder() *Founder {
	return fnd
}

// Start find files
func (f *Founder) Start() error {
	go f.parser.Start()

	f.findInitial()

	go func() {
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
	}()

	err := f.watcher.Add(f.config.DumpPath)
	if err != nil {
		return err
	}

	return nil
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

// some_name_20060102_150405.pcap
func (f *Founder) matchRegexp(file string) bool {
	_, name := filepath.Split(file)

	if len(name) <= 20 {
		return false
	}

	return f.regexp.MatchString(name)
}
