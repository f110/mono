package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"golang.org/x/xerrors"

	"go.f110.dev/mono/bot/continuous-delivery/pkg/config"
	"go.f110.dev/mono/bot/continuous-delivery/pkg/consumer"
	"go.f110.dev/mono/bot/continuous-delivery/pkg/webhook"
	"go.f110.dev/mono/lib/logger"
)

func producer(args []string) error {
	confFile := ""
	buildRuleFile := ""
	debug := false
	fs := pflag.NewFlagSet("maintenance-bot", pflag.ContinueOnError)
	fs.StringVarP(&confFile, "conf", "c", confFile, "Config file")
	fs.StringVar(&buildRuleFile, "build-rule", buildRuleFile, "Build rule")
	fs.BoolVarP(&debug, "debug", "D", debug, "Debug")
	if err := fs.Parse(args); err != nil {
		return xerrors.Errorf(": %v", err)
	}

	conf, err := config.ReadConfig(confFile)
	if err != nil {
		return xerrors.Errorf(": %v", err)
	}

	if err := logger.Init(); err != nil {
		return xerrors.Errorf(": %v", err)
	}

	webhookListener := webhook.NewListener(conf)

	builder, err := consumer.NewBuildConsumer(conf.BuildNamespace, conf, debug)
	if err != nil {
		return xerrors.Errorf(": %v", err)
	}
	webhookListener.SubscribePushEvent(builder.Build)

	dnsControlBuilder, err := consumer.NewDNSControlConsumer(conf.BuildNamespace, conf, conf.SafeMode, debug)
	if err != nil {
		return xerrors.Errorf(": %v", err)
	}
	webhookListener.SubscribePushEvent(dnsControlBuilder.Dispatch)
	webhookListener.SubscribePullRequest(dnsControlBuilder.Dispatch)

	logger.Log.Info("Start server", zap.String("addr", conf.WebhookListener))
	if err := webhookListener.ListenAndServe(); err != nil {
		if err == http.ErrServerClosed {
			return nil
		}

		return xerrors.Errorf(": %v", err)
	}

	return nil
}

func main() {
	if err := producer(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
