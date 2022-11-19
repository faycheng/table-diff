package main

import (
	"os"

	"github.com/sirupsen/logrus"
)

var log = logrus.New()

func init() {
	log.SetOutput(os.Stdout)
	log.SetLevel(logrus.InfoLevel)
	if os.Getenv("DEBUG") == "true" {
		log.SetLevel(logrus.DebugLevel)
		log.SetReportCaller(true)
	}
}
