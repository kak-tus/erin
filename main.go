package main

import (
	"bytes"

	"github.com/fiorix/go-smpp/smpp/pdu"
	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
)

func main() {
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

		// lst := bdy.FieldList()
		// for _, l := range lst {
		// 	println(l)
		// }

		println(bdy.Header().ID.String())
		println(bdy.Header().Status.Error())
	}
}
