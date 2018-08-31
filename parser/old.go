package parser

import (
	"os"
	"path/filepath"
	"time"
)

func (p *Parser) moveOld() {
	now := time.Now()

	for file := range p.toMove {
		_, name := filepath.Split(file)

		// dump_connectionID_20060102_150405.pcap
		timeFromName := name[len(name)-20 : len(name)-5]

		tm, err := time.ParseInLocation("20060102_150405", timeFromName, time.Local)
		if err != nil {
			p.logger.Error(err)
			continue
		}

		if now.Sub(tm).Hours() <= 2 {
			continue
		}

		newPath := filepath.Join(p.config.MovePath, name)
		err = os.Rename(file, newPath)
		if err != nil {
			p.logger.Error(err)
			continue
		}

		delete(p.toMove, file)
	}
}
