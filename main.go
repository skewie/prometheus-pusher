package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/kardianos/service"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/retrieval"
	"github.com/prometheus/prometheus/storage"
	"github.com/skewie/prometheus-pusher/scrape"
)

var cfg = struct {
	configFile        string
	customLabels      string
	customLabelValues string
	port              uint
	pushgatewayUrl    string
}{}

var (
	labels, values []string
)

func init() {
	flag.StringVar(
		&cfg.configFile, "config.file", "prometheus_pusher.yml",
		"Prometheus configuration file name.",
	)
	flag.StringVar(
		&cfg.customLabels, "config.customLabels", "", "custom metrics labels",
	)
	flag.StringVar(
		&cfg.customLabelValues, "config.customLabelValues", "", "custom mertics label values",
	)
	flag.UintVar(
		&cfg.port, "port", 8082, "The port that the application will listen on.",
	)
	flag.StringVar(
		&cfg.pushgatewayUrl, "pgUrl", "localhost:9091", "The PushGateway URL to push metrics to",
	)
}

var logger service.Logger

type program struct{}

func (p *program) Start(s service.Service) error {
	// Start should not block. Do the actual work async.
	go p.run()
	return nil
}
func (p *program) run() {
	flag.Parse()
	var (
		sampleAppender = storage.Fanout{}
		targetManager  = retrieval.NewTargetManager(sampleAppender)
		jobTargets     = scrape.NewJobTargets(targetManager)
	)

	logger.Info("Loading prometheus config file: " + cfg.configFile)
	logger.Info("Custom labels: " + cfg.customLabels + "\t Custom label values: " + cfg.customLabelValues)

	if cfg.customLabels == "" {
		labels = []string{}
		values = []string{}
	} else {
		labels = strings.Split(cfg.customLabels, ",")
		values = strings.Split(cfg.customLabelValues, ",")
	}

	scrape.SetPushGateway(cfg.pushgatewayUrl)

	var (
		scrapeManager = scrape.NewExporterScrape(jobTargets, labels, values)
	)

	conf, err := config.LoadFile(cfg.configFile)
	if err != nil {
		logger.Error(err.Error())
		return
	}

	targetManager.ApplyConfig(conf)

	go targetManager.Run()
	defer targetManager.Stop()

	scrapeManager.AppConfig(conf)

	go scrapeManager.Run()
	defer scrapeManager.Stop()

	r := gin.Default()
	pprof.Register(r, nil) // NOQA

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	r.GET("/targets", func(c *gin.Context) {
		c.JSON(200, jobTargets.Targets())
	})
	r.Run(fmt.Sprintf(":%v", cfg.port))
}

func (p *program) Stop(s service.Service) error {
	// Stop should not block. Return with a few seconds.
	return nil
}

func main() {
	flag.Parse()
	svcConfig := &service.Config{
		Name:        "GoServiceExampleSimple",
		DisplayName: "Go Service Example",
		Description: "This is an example Go service.",
	}

	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		logger.Error(err)
	}
	logger, err = s.Logger(nil)
	if err != nil {
		logger.Error(err)
	}
	err = s.Run()
	if err != nil {
		logger.Error(err)
	}
}
