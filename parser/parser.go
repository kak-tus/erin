package parser

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/peterbourgon/diskv"

	"git.aqq.me/go/app/appconf"
	"git.aqq.me/go/app/applog"
	"git.aqq.me/go/app/event"
	"github.com/fiorix/go-smpp/smpp/pdu"
	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"github.com/mitchellh/mapstructure"
)

var prs *Parser

func init() {
	event.Init.AddHandler(
		func() error {
			cnfMap := appconf.GetConfig()["parser"]

			var cnf parserConfig
			err := mapstructure.Decode(cnfMap, &cnf)
			if err != nil {
				return err
			}

			d := diskv.New(diskv.Options{
				BasePath: cnf.StorePath,
			})

			prs = &Parser{
				logger: applog.GetLogger(),
				m:      &sync.Mutex{},
				C:      make(chan string, 1000),
				toMove: make(map[string]bool),
				config: cnf,
				diskv:  d,
			}

			prs.logger.Info("Started parser")

			return nil
		},
	)

	event.Stop.AddHandler(
		func() error {
			prs.logger.Info("Stop parser")
			close(prs.C)
			prs.m.Lock()
			return nil
		},
	)
}

// GetParser return instance
func GetParser() *Parser {
	return prs
}

// Start parser
func (p *Parser) Start() {
	p.m.Lock()

	for {
		file, more := <-p.C

		if !more {
			break
		}

		p.parse(file)
	}

	p.m.Unlock()
}

func (p *Parser) parse(file string) {
	p.logger.Debug(file)
	p.toMove[file] = true

	p.moveOld()
}

func (p *Parser) parse1() {
	hdl, err := pcap.OpenOffline("dump.pcap")
	if err != nil {
		panic(err)
	}

	src := gopacket.NewPacketSource(hdl, hdl.LinkType())

	for pac := range src.Packets() {
		appl := pac.ApplicationLayer()
		if appl == nil {
			continue
		}

		rdr := bytes.NewReader(appl.LayerContents())

		bdy, err := pdu.Decode(rdr)
		if err != nil {
			continue
		}

		lst := bdy.FieldList()
		for _, l := range lst {
			println(l)
		}

		println(bdy.Header().ID.String())
		println(bdy.Header().Status.Error())
	}
}

func (p *Parser) moveOld() {
	now := time.Now()

	for file := range p.toMove {
		_, name := filepath.Split(file)

		tm, err := time.ParseInLocation("dump_20060102_150405.pcap", name, time.Local)
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
