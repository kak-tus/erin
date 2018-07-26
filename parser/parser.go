package parser

import (
	"bytes"
	"sync"

	"git.aqq.me/go/app/applog"
	"git.aqq.me/go/app/event"
	"github.com/fiorix/go-smpp/smpp/pdu"
	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
)

var prs *Parser

func init() {
	event.Init.AddHandler(
		func() error {
			prs = &Parser{
				logger: applog.GetLogger(),
				m:      &sync.Mutex{},
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
	p.C = make(chan string, 1000)
	p.m.Lock()

	for {
		file, more := <-p.C

		if !more {
			break
		}

		p.logger.Debug(file)
	}

	p.m.Unlock()
}

func (p *Parser) parse() {
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
