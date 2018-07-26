package founder

import (
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

	go func() {
	LOOP:
		for {
			select {
			case ev, more := <-f.watcher.Events:
				if !more {
					break LOOP
				}

				f.parser.C <- ev.Name
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
