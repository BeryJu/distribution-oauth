package main

import (
	"beryju.org/distribution-oauth/internal"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetLevel(log.TraceLevel)
	log.SetFormatter(&log.JSONFormatter{
		DisableHTMLEscape: true,
	})
	s := internal.New()
	s.Run()
}
