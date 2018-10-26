package parser

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"git.aqq.me/go/app/appconf"
	"git.aqq.me/go/app/applog"
	"git.aqq.me/go/app/event"
	"git.aqq.me/go/nanachi"
	"git.aqq.me/go/retrier"
	"github.com/go-redis/redis"
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

			loc, err := time.LoadLocation("UTC")
			if err != nil {
				return err
			}

			addrs := strings.Split(cnf.Redis.Addrs, ",")

			redisdb := redis.NewClusterClient(&redis.ClusterOptions{
				Addrs:    addrs,
				Password: cnf.Redis.Password,
			})

			prs = &Parser{
				logger:   applog.GetLogger().Sugar(),
				m:        &sync.Mutex{},
				C:        make(chan string, 1000),
				toMove:   make(map[string]bool),
				config:   cnf,
				diskv:    d,
				location: loc,
				redisdb:  redisdb,
				retrier:  retrier.New(retrier.Config{RetryPolicy: []time.Duration{time.Second * 5}}),
			}

			prs.logger.Info("Started parser")

			return nil
		},
	)

	event.Stop.AddHandler(
		func() error {
			prs.logger.Info("Stop parser")
			prs.m.Lock()

			prs.nanachi.Close()

			err := prs.redisdb.Close()
			if err != nil {
				return err
			}

			prs.logger.Info("Stopped parser")
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
			URI:       p.config.URI,
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

	producer := client.NewSmartProducer(
		nanachi.SmartProducerConfig{
			Destinations:      []*nanachi.Destination{dest},
			Confirm:           true,
			Mandatory:         true,
			PendingBufferSize: 1000,
		},
	)

	p.nanachi = client
	p.producer = producer

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

// Stop parser
func (p *Parser) Stop() {
	close(p.C)
}
