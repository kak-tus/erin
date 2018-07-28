package parser

import (
	"bytes"
	"path/filepath"
	"strconv"
	"sync"

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

		p.process(file)
	}

	p.m.Unlock()
}

func (p *Parser) process(file string) {
	p.logger.Debug(file)
	p.toMove[file] = true

	p.parse(file)

	p.moveOld()
}

func (p *Parser) parse(file string) {
	_, name := filepath.Split(file)

	offset := 0

	if p.diskv.Has(name) {
		raw, err := p.diskv.Read(name)
		if err != nil {
			p.logger.Error(err)
			return
		}
		offset, err = strconv.Atoi(string(raw))
		if err != nil {
			p.logger.Error(err)
			return
		}
	}

	hdl, err := pcap.OpenOffline(file)
	if err != nil {
		p.logger.Error(err)
		return
	}

	src := gopacket.NewPacketSource(hdl, hdl.LinkType())
	count := 0

	for pac := range src.Packets() {
		count++

		if offset >= count {
			continue
		}

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

	err = p.diskv.Write(name, []byte(strconv.Itoa(count)))
	if err != nil {
		p.logger.Error(err)
		return
	}
}
