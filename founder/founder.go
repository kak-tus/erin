package founder

import (
	"git.aqq.me/go/app/appconf"
	"git.aqq.me/go/app/applog"
	"git.aqq.me/go/app/event"
	"github.com/fsnotify/fsnotify"
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

			logger := applog.GetLogger()

			wtch, err := fsnotify.NewWatcher()
			if err != nil {
				return err
			}

			fnd = &Founder{
				logger:  logger,
				config:  cnf,
				watcher: wtch,
			}

			return nil
		},
	)

	event.Stop.AddHandler(
		func() error {
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
	go func() {
		for {
			select {
			case ev := <-f.watcher.Events:
				f.logger.Debug(ev.Op.String())
			case err := <-f.watcher.Errors:
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
