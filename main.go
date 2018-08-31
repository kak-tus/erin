package main

import (
	"git.aqq.me/go/app/appconf"
	"git.aqq.me/go/app/launcher"
	"github.com/iph0/conf/envconf"
	"github.com/iph0/conf/fileconf"
	"github.com/kak-tus/erin/founder"
	"github.com/kak-tus/healthcheck"
)

func init() {
	fileLdr := fileconf.NewLoader("etc")

	envLdr := envconf.NewLoader()

	appconf.RegisterLoader("file", fileLdr)
	appconf.RegisterLoader("env", envLdr)

	appconf.Require("file:erin.yml")
	appconf.Require("env:^ERIN_")
}

func main() {
	var fnd *founder.Founder

	launcher.Run(func() error {
		healthcheck.Add("/healthcheck", func() (healthcheck.State, string) {
			return healthcheck.StatePassing, "ok"
		})

		fnd = founder.GetFounder()

		err := fnd.Start()
		if err != nil {
			return err
		}

		return nil
	})
}
