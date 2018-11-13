package parser

import (
	"strings"
	"sync"
	"time"

	"git.aqq.me/go/app/appconf"
	"git.aqq.me/go/app/applog"
	"git.aqq.me/go/app/event"
	"git.aqq.me/go/retrier"
	"github.com/go-redis/redis"
	"github.com/kak-tus/ami"
	"github.com/peterbourgon/diskv"
)

var prs *Parser

func init() {
	event.Init.AddHandler(
		func() error {
			cnfMap := appconf.GetConfig()["parser"]

			var cnf parserConfig
			err := appconf.Decode(cnfMap, &cnf)
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

			ropt := &redis.ClusterOptions{
				Addrs:    addrs,
				Password: cnf.Redis.Password,
			}

			redisdb := redis.NewClusterClient(ropt)

			pr, err := ami.NewProducer(
				ami.ProducerOptions{
					Name:              cnf.QueueName,
					ShardsCount:       cnf.ShardsCount,
					PendingBufferSize: cnf.PendingBufferSize,
					PipeBufferSize:    cnf.PipeBufferSize,
					PipePeriod:        time.Microsecond * 10,
				},
				ropt,
			)
			if err != nil {
				return err
			}

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
				pr:       pr,
			}

			prs.logger.Info("Started parser")

			return nil
		},
	)

	event.Stop.AddHandler(
		func() error {
			prs.logger.Info("Stop parser")
			prs.m.Lock()

			prs.pr.Close()

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
