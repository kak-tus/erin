package parser

import (
	"os"
	"path/filepath"
	"time"
)

func (p *Parser) moveOld() {
	now := time.Now()

	for file := range p.toMove {
		// Check if file moved already
		_, err := os.Stat(file)
		if err != nil && os.IsNotExist(err) {
			delete(p.toMove, file)
			continue
		}

		_, name := filepath.Split(file)

		// connectionName_connectionID_20060102_150405.pcap
		timeFromName := name[len(name)-20 : len(name)-5]

		tm, err := time.ParseInLocation("20060102_150405", timeFromName, time.Local)
		if err != nil {
			p.logger.Error(err)
			continue
		}

		if now.Sub(tm).Hours() <= 2 {
			continue
		}

		noTime := name[0 : len(name)-21]

		part1 := tm.Format("20060102")
		part2 := tm.Format("150405") + ".pcap"

		newDir := filepath.Join(p.config.MovePath, noTime, part1)

		err = os.MkdirAll(newDir, 0775)
		if err != nil {
			p.logger.Error(err)
			continue
		}

		newPath := filepath.Join(newDir, part2)

		err = os.Rename(file, newPath)
		if err != nil {
			p.logger.Error(err)
			continue
		}

		delete(p.toMove, file)
	}
}
