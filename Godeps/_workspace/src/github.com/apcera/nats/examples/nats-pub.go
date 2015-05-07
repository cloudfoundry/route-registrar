// Copyright 2012 Apcera Inc. All rights reserved.
// +build ignore

package main

import (
	"flag"
	"log"
	"strings"

	"github.com/apcera/nats"
)

func usage() {
	log.Fatalf("Usage: nats-pub [-s server] [--ssl] [-t] <subject> <msg> \n")
}

func main() {
	var urls = flag.String("s", nats.DefaultURL, "The nats server URLs (separated by comma)")
	var ssl = flag.Bool("ssl", false, "Use Secure Connection")

	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		usage()
	}

	opts := nats.DefaultOptions
	opts.Servers = strings.Split(*urls, ",")
	for i, s := range opts.Servers {
		opts.Servers[i] = strings.Trim(s, " ")
	}

	opts.Secure = *ssl

	nc, err := opts.Connect()
	if err != nil {
		log.Fatalf("Can't connect: %v\n", err)
	}

	subj, msg := args[0], []byte(args[1])

	nc.Publish(subj, msg)
	nc.Close()

	log.Printf("Published [%s] : '%s'\n", subj, msg)
}
