// Copyright 2017 Jeff Foley. All rights reserved.
// Use of this source code is governed by Apache 2 LICENSE that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"
	"path"
	"sync"
	"time"

	"github.com/OWASP/Amass/amass"
	"github.com/OWASP/Amass/amass/dnssrv"
	"github.com/OWASP/Amass/amass/utils"
)

var (
	wg      sync.WaitGroup
	results chan string
)

func main() {
	var addrs parseIPs
	var cidrs parseCIDRs
	var asns, ports parseInts
	results = make(chan string, 50)

	help := flag.Bool("h", false, "Show the program usage message")
	flag.Var(&addrs, "addr", "IPs and ranges (192.168.1.1-254) separated by commas")
	flag.Var(&cidrs, "cidr", "CIDRs separated by commas (can be used multiple times)")
	flag.Var(&asns, "asn", "ASNs separated by commas (can be used multiple times)")
	flag.Var(&ports, "p", "Ports separated by commas (default: 443)")
	flag.Parse()

	if *help {
		fmt.Printf("Usage: %s [--addr IP] [--cidr CIDR] [--asn number] [-p number]\n", path.Base(os.Args[0]))
		flag.PrintDefaults()
		return
	}

	if len(ports) == 0 {
		ports = []int{443}
	}

	rand.Seed(time.Now().UTC().UnixNano())

	ips := AllIPsInScope(addrs, cidrs, asns)
	if len(ips) == 0 {
		fmt.Println("The parameters identified no hosts")
		return
	}

	go PullAllCertificates(ips, ports)
	go ReverseDNS(ips)
	go UniquePrint()
	// Wait for DNS queries and certificate pulls
	wg.Add(2)
	wg.Wait()
	// Wait for all the prints to finish
	wg.Add(1)
	close(results)
	wg.Wait()
}

func UniquePrint() {
	filter := make(map[string]struct{})

	for domain := range results {
		if _, found := filter[domain]; domain != "" && !found {
			filter[domain] = struct{}{}
			fmt.Println(domain)
		}
	}
	wg.Done()
}

func AllIPsInScope(addrs parseIPs, cidrs parseCIDRs, asns parseInts) []net.IP {
	var ips []net.IP

	ips = append(ips, addrs...)

	for _, cidr := range cidrs {
		ips = append(ips, utils.NetHosts(cidr)...)
	}

	for _, asn := range asns {
		record, err := amass.ASNRequest(asn)
		if err != nil {
			continue
		}

		for _, cidr := range record.Netblocks {
			_, ipnet, err := net.ParseCIDR(cidr)
			if err != nil {
				continue
			}

			ips = append(ips, utils.NetHosts(ipnet)...)
		}
	}
	return ips
}

func ReverseDNS(ips []net.IP) {
	for _, ip := range ips {
		if _, answer, err := dnssrv.Reverse(ip.String()); err == nil {
			results <- amass.SubdomainToDomain(answer)
		}
		time.Sleep(time.Millisecond)
	}
	wg.Done()
}

func PullAllCertificates(ips []net.IP, ports parseInts) {
	var running, idx int
	done := make(chan struct{}, 100)

	t := time.NewTicker(25 * time.Millisecond)
	defer t.Stop()
loop:
	for {
		select {
		case <-t.C:
			if running >= 100 || idx >= len(ips) {
				continue
			}

			running++
			addr := ips[idx]
			go ObtainCert(addr.String(), ports, done)
			idx++
		case <-done:
			running--
			if running == 0 && idx >= len(ips) {
				break loop
			}
		}
	}
	wg.Done()
}

func ObtainCert(addr string, ports parseInts, done chan struct{}) {
	var domains []string

	for _, r := range amass.PullCertificateNames(addr, ports) {
		domains = utils.UniqueAppend(domains, r.Domain)
	}

	for _, domain := range domains {
		results <- domain
	}
	done <- struct{}{}
}
