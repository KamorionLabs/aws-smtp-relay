package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/KamorionLabs/aws-smtp-relay/internal/relay/config"
	"github.com/KamorionLabs/aws-smtp-relay/internal/relay/server"
	"github.com/spf13/pflag"
)

func dumpCfg(cfg *config.Config) {
	entry := struct {
		Time      time.Time
		Component string
		Cfg       interface{}
	}{
		Time:      time.Now().UTC(),
		Component: "aws-smtp-relay",
		Cfg:       cfg,
	}
	b, _ := json.Marshal(entry)
	fmt.Println(string(b))
}

func main() {
	pflag.Parse()
	cfg, err := config.Configure()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	dumpCfg(cfg)

	srv, err := server.Server(cfg)
	if err == nil {
		err = srv.ListenAndServe()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
