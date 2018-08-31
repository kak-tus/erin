package parser

import (
	"fmt"
	"sync"
	"time"

	"git.aqq.me/go/app/appconf"
	"git.aqq.me/go/app/applog"
	"git.aqq.me/go/app/event"
	"git.aqq.me/go/nanachi"
	"git.aqq.me/go/retrier"
	"github.com/mitchellh/mapstructure"
	"github.com/peterbourgon/diskv"
	"github.com/streadway/amqp"
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
