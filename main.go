package main

import (
	"bufio"
	"context"
	"fmt"
	flags "github.com/jessevdk/go-flags"
	"net"
	"os"
	"strings"
	"sync"
)

var opts struct {
	Threads     int    `short:"t" long:"threads" default:"8" description:"How many threads should be used"`
	ResolverIP  string `short:"r" long:"resolver" description:"IP of the DNS resolver to use for lookups"`
	ResolverIPs string `short:"l" long:"resolvers" description:"IPs of the DNS resolvers to use for lookups, comma delimited"`
	Protocol    string `short:"P" long:"protocol" choice:"tcp" choice:"udp" default:"udp" description:"Protocol to use for lookups"`
	Port        uint16 `short:"p" long:"port" default:"53" description:"Port to bother the specified DNS resolver on"`
	Domain      bool   `short:"d" long:"domain" description:"Output only domains"`
}

func main() {
	_, err := flags.ParseArgs(&opts, os.Args)
	if err != nil {
		os.Exit(1)
	}

	// default of 8 threads
	numWorkers := opts.Threads

	work := make(chan string)
	go func() {
		s := bufio.NewScanner(os.Stdin)
		for s.Scan() {
			work <- s.Text()
		}
		close(work)
	}()

	wg := &sync.WaitGroup{}

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go doWork(work, wg)
	}
	wg.Wait()
}

func getResolvers() []*net.Resolver {
	resolvers := make([]*net.Resolver, 0)
	var r *net.Resolver

	if opts.ResolverIP != "" {
		r = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{}
				return d.DialContext(ctx, opts.Protocol, fmt.Sprintf("%s:%d", opts.ResolverIP, opts.Port))
			},
		}
	} else if opts.ResolverIPs != "" {
		ips := strings.Split(opts.ResolverIPs, ",")

		for _, ip := range ips {
			resolver := &net.Resolver{
				PreferGo: true,
				Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
					d := net.Dialer{}
					return d.DialContext(ctx, opts.Protocol, fmt.Sprintf("%s:%d", ip, opts.Port))
				},
			}

			resolvers = append(resolvers, resolver)
		}
	}

	if len(resolvers) == 0 {
		resolvers = append(resolvers, r)
	}

	return resolvers
}

func doWork(work chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	resolvers := getResolvers()

	for ip := range work {
		for _, r := range resolvers {
			addr, err := r.LookupAddr(context.Background(), ip)
			if err != nil {
				continue
			}

			for _, a := range addr {
				if opts.Domain {
					fmt.Println(strings.TrimRight(a, "."))
				} else {
					fmt.Println(ip, "\t", strings.TrimRight(a, "."))
				}
			}
		}
	}
}
