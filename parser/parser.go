package parser

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fiorix/go-smpp/smpp/pdu/pdutext"

	"git.aqq.me/go/app/appconf"
	"git.aqq.me/go/app/applog"
	"git.aqq.me/go/app/event"
	"git.aqq.me/go/nanachi"
	"git.aqq.me/go/retrier"
	"github.com/fiorix/go-smpp/smpp/pdu"
	"github.com/fiorix/go-smpp/smpp/pdu/pdufield"
	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"github.com/kak-tus/corrie/message"
	"github.com/mitchellh/mapstructure"
	"github.com/peterbourgon/diskv"
	"github.com/streadway/amqp"
)

var prs *Parser

const sql = "INSERT INTO gate.kannel " +
	"(dt_part,dt,connection_id,operation,type,status,sequence_id,message_id,phone,short_number,txt)	" +
	"VALUES (?,?,?,?,?,?,?,?,?,?,?);"

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
				logger: applog.GetLogger().Sugar(),
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
	client, err := nanachi.NewClient(
		nanachi.ClientConfig{
			URI:       "amqp://example:example@example.com:5672/example",
			Heartbeat: time.Second * 15,
			RetrierConfig: &retrier.Config{
				RetryPolicy: []time.Duration{time.Second},
			},
		},
	)

	if err != nil {
		p.logger.Error(err)
		return
	}

	dest := &nanachi.Destination{
		RoutingKey: p.config.QueueName,
		MaxShard:   p.config.MaxShard,
		Declare: func(ch *amqp.Channel) error {
			for i := 0; i <= int(p.config.MaxShard); i++ {
				shardName := fmt.Sprintf("%s.%d", p.config.QueueName, i)

				_, err := ch.QueueDeclare(shardName, true, false, false, false, nil)
				if err != nil {
					panic(err)
				}
			}

			return nil
		},
	}

	p.nanachi = client
	p.dest = dest

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
		raw := p.diskv.ReadString(name)

		var err error

		offset, err = strconv.Atoi(raw)
		if err != nil {
			p.logger.Error(err)
			return
		}
	}

	// dump_connectionID_20060102_150405.pcap
	str := name[strings.LastIndex(name[0:len(name)-21], "_")+1 : len(name)-21]
	connectionID, err := strconv.Atoi(str)
	if err != nil {
		p.logger.Error(err)
		return
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

		data := make([]interface{}, 11)

		meta := pac.Metadata()
		header := bdy.Header()
		fields := bdy.Fields()

		if !(header.ID == pdu.SubmitSMID || header.ID == pdu.DeliverSMID) {
			continue
		}

		data[0] = meta.Timestamp.Format("2006-01-02")          // dt_part
		data[1] = meta.Timestamp.Format("2006-01-02 15:04:05") // dt
		data[2] = connectionID                                 // connection_id
		data[3] = header.ID.String()                           // operation
		data[4] = getMsgType(header, fields)                   // type
		data[5] = header.Status.Error()                        // status
		data[6] = header.Seq                                   // sequence_id

		// message_id
		messageID := fields["message_id"]
		if messageID != nil {
			data[7] = messageID.String()
		}

		// phone,short_number
		var phone pdufield.Body
		var shortNumber pdufield.Body

		if header.ID == pdu.SubmitSMID {
			phone = fields["destination_addr"]
			shortNumber = fields["source_addr"]
		} else if header.ID == pdu.DeliverSMID {
			phone = fields["source_addr"]
			shortNumber = fields["destination_addr"]
		}

		if phone != nil {
			data[8] = phone.String()
		}
		if shortNumber != nil {
			data[9] = shortNumber.String()
		}

		// txt
		data[10] = getText(header, fields)

		if data[10] == nil {
			continue
		}

		body, err := message.Message{
			Query: sql,
			Data:  data,
		}.Encode()

		p.logger.Debug(string(body))
	}

	// err = p.diskv.Write(name, []byte(strconv.Itoa(count)))
	// if err != nil {
	// 	p.logger.Error(err)
	// 	return
	// }
}

func getText(header *pdu.Header, fields pdufield.Map) interface{} {
	if !(header.ID == pdu.SubmitSMID || header.ID == pdu.DeliverSMID) {
		return nil
	}

	if header.ID == pdu.DeliverSMID {
		esmClass, ok := fields["esm_class"]

		if ok && esmClass.String() == "4" {
			return nil
		}
	}

	txt, ok := fields["short_message"]
	if !ok {
		return nil
	}

	coding, ok := fields["data_coding"]
	if !ok {
		return string(pdutext.GSM7(txt.Bytes()).Decode())
	}

	raw, err := strconv.Atoi(coding.String())
	if err != nil {
		return nil
	}

	c := pdutext.DataCoding(raw)

	var enc []byte

	if c == pdutext.ISO88595Type {
		enc = pdutext.ISO88595(txt.Bytes()).Decode()
	} else if c == pdutext.Latin1Type {
		enc = pdutext.Latin1(txt.Bytes()).Decode()
	} else if c == pdutext.UCS2Type {
		enc = pdutext.UCS2(txt.Bytes()).Decode()
	} else {
		enc = pdutext.GSM7(txt.Bytes()).Decode()
	}

	return string(enc)
}

func getMsgType(header *pdu.Header, fields pdufield.Map) interface{} {
	if header.ID == pdu.SubmitSMID {
		return "sms"
	} else if header.ID == pdu.DeliverSMID {
		esmClass, ok := fields["esm_class"]

		if ok && esmClass.String() == "4" {
			return "dlr"
		}

		return "sms"
	}

	return nil
}
