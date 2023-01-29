package main

import (
	"os"
	"strings"

	"beryju.org/distribution-oauth/internal"
	log "github.com/sirupsen/logrus"
)

func main() {
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "trace":
		log.SetLevel(log.TraceLevel)
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warning":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	default:
		log.SetLevel(log.DebugLevel)
	}
	log.SetFormatter(&log.JSONFormatter{
		DisableHTMLEscape: true,
	})
	s := internal.New()
	s.Run()
}
