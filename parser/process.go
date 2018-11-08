package parser

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"git.aqq.me/go/retrier"
	"github.com/fiorix/go-smpp/smpp/pdu"
	"github.com/fiorix/go-smpp/smpp/pdu/pdufield"
	"github.com/fiorix/go-smpp/smpp/pdu/pdutext"
	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"github.com/kak-tus/ruthie/message"
)

const sql = "INSERT INTO gate.smpp " +
	"(dt_part,dt,connection_id,operation,type,status,sequence_id,message_id,phone,short_number,txt_id) " +
	"VALUES (?,?,?,?,?,?,?,?,?,?,?);"

const sqlText = "INSERT INTO gate.smpp_texts (dt_part,id,txt) VALUES (?,?,?);"

func (p *Parser) process(file string) {
	p.logger.Debug("Got file ", file)
	p.toMove[file] = true

	p.parse(file)

	p.moveOld()
}

func (p *Parser) parse(file string) {
	_, name := filepath.Split(file)

	offset := 0

	rdKey := fmt.Sprintf("{erin_%s}", name)

	if p.diskv.Has(name) {
		raw := p.diskv.ReadString(name)

		var err error

		offset, err = strconv.Atoi(raw)
		if err != nil {
			p.logger.Error(err)
			return
		}
	}

	// connectionName_connectionID_20060102_150405.pcap
	noTime := name[0 : len(name)-21]
	str := noTime[strings.LastIndex(noTime, "_")+1:]
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

	defer hdl.Close()

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

		inGMT := meta.Timestamp.In(p.location)

		data[0] = inGMT.Format("2006-01-02")          // dt_part
		data[1] = inGMT.Format("2006-01-02 15:04:05") // dt
		data[2] = connectionID                        // connection_id
		data[3] = header.ID.String()                  // operation
		data[4] = getMsgType(header, fields)          // type
		data[5] = header.Status                       // status
		data[6] = header.Seq                          // sequence_id

		// message_id
		messageID := fields["message_id"]
		if messageID != nil {
			data[7] = messageID.String()
		}

		var phone pdufield.Body
		var shortNumber pdufield.Body

		if header.ID == pdu.SubmitSMID {
			phone = fields["destination_addr"]
			shortNumber = fields["source_addr"]
		} else if header.ID == pdu.DeliverSMID {
			phone = fields["source_addr"]
			shortNumber = fields["destination_addr"]
		}

		// phone
		if phone != nil {
			data[8] = phone.String()
		}

		// short_number
		if shortNumber != nil {
			data[9] = shortNumber.String()
		}

		txt := getText(header, fields)

		// txt_id
		if txt != nil {
			sha := sha1.Sum([]byte(txt.(string)))
			data[10] = string(sha[:])

			dataText := make([]interface{}, 3)

			dataText[0] = data[0]  // dt_part
			dataText[1] = data[10] // id
			dataText[2] = txt      // txt

			body, err := message.Message{
				Query: sqlText,
				Data:  dataText,
			}.Encode()

			if err == nil {
				p.pr.Send(body)
			}
		}

		body, err := message.Message{
			Query: sql,
			Data:  data,
		}.Encode()
		if err != nil {
			continue
		}

		p.pr.Send(body)
	}

	p.retrier.Do(func() *retrier.Error {
		err = p.diskv.WriteString(name, strconv.Itoa(count))
		if err != nil {
			p.logger.Error(err)
			return retrier.NewError(err, false)
		}

		status := p.redisdb.Set(rdKey, count, redisTTL)
		if status.Err() != nil {
			p.logger.Error(status.Err())
			return retrier.NewError(err, false)
		}

		return nil
	})

	p.logger.Debug("Processing done ", file)
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

	if len(txt.Bytes()) == 0 {
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
