package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/AdguardTeam/dnsproxy/upstream"
	"github.com/miekg/dns"
	"golang.org/x/exp/constraints"
)

const N = 4

// const Interval = time.Duration(1) * time.Second
const Timeout = time.Duration(5) * time.Second
const MaxDuration = time.Duration(math.MaxInt64)

type Pinger struct {
	name          string
	server        string
	comments      []string
	minDuration   time.Duration
	maxDuration   time.Duration
	totalDuration time.Duration
}

func NewPinger(name string) *Pinger {
	// TODO name strip ##
	return &Pinger{
		name:          name,
		server:        "",
		minDuration:   MaxDuration,
		maxDuration:   time.Duration(0) * time.Millisecond,
		totalDuration: time.Duration(0) * time.Millisecond,
	}
}

func (p *Pinger) Summarize() {
	fmt.Printf(
		"%s \n Minimum = %s, Maximum = %s, Average = %s\n",
		p.name,
		p.minDuration,
		p.maxDuration,
		p.totalDuration/time.Duration(N),
	)
}

func main() {
	dnscryptPtr := flag.Bool("dnscrypt", false, "a bool")
	dohPtr := flag.Bool("doh", false, "a bool")
	flag.Parse()

	file, err := os.Open("public-resolvers.md")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	pingers := make([]Pinger, 0)
	current := NewPinger("")

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "##") {
			if current.server != "" {
				pingers = append(pingers, *current)
			}
			current = NewPinger(line)
		} else if strings.HasPrefix(line, "sdns://") {
			current.server = line
		} else {
			// TODO skip blank line
			current.comments = append(current.comments, line)
		}
	}
	if current.server != "" {
		pingers = append(pingers, *current)
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	if *dnscryptPtr {
		DNScrypt_pingers := make([]Pinger, 0)
		for _, pinger := range pingers {
			for _, comment := range pinger.comments {
				if strings.Contains(comment, "DNSCrypt") {
					DNScrypt_pingers = append(DNScrypt_pingers, pinger)
					break
				}
			}
		}
		pingers = DNScrypt_pingers
	}

	if *dohPtr {
		doh_pingers := make([]Pinger, 0)
		for _, pinger := range pingers {
			for _, comment := range pinger.comments {
				if strings.Contains(comment, "DoH") {
					doh_pingers = append(doh_pingers, pinger)
					break
				}
			}
		}
		pingers = doh_pingers
	}

	fmt.Println(len(pingers))

	validPingers := make([]Pinger, 0)
	for _, pinger := range pingers {
		// fmt.Println(pinger.name)
		err := pinger.testSpeed()
		if err == nil {
			validPingers = append(validPingers, pinger)
		}
	}

	sort.Slice(validPingers, func(i, j int) bool {
		return validPingers[i].totalDuration < validPingers[j].totalDuration
	})

	fmt.Println(len(validPingers))
	for i := 0; i < min(10, len(validPingers)); i++ {
		validPingers[i].Summarize()
	}

}

func By(totalDuratioin func(p1 *Pinger, p2 *Pinger) bool) {
	panic("unimplemented")
}

func (p *Pinger) testSpeed() error {
	for i := 0; i < N; i++ {
		d, err := google(p.server)
		if err != nil {
			return err
		}
		p.totalDuration += d
		p.maxDuration = max(p.maxDuration, d)
		p.minDuration = min(p.minDuration, d)
	}
	return nil
}

func google(server string) (time.Duration, error) {
	domain := "google.com"
	rrType := dns.TypeA

	// true: no verify
	// false: verify
	insecureSkipVerify := false

	var httpVersions []upstream.HTTPVersion
	/*
		if http3Enabled {
			httpVersions = []upstream.HTTPVersion{
				upstream.HTTPVersion3,
				upstream.HTTPVersion2,
				upstream.HTTPVersion11,
			}
		}
	*/

	var class uint16
	class = dns.ClassINET

	opts := &upstream.Options{
		Timeout:            Timeout,
		InsecureSkipVerify: insecureSkipVerify,
		HTTPVersions:       httpVersions,
	}

	u, err := upstream.AddressToUpstream(server, opts)
	if err != nil {
		log.Fatalf("Cannot create an upstream: %s", err)
	}

	req := dns.Msg{}
	req.Id = dns.Id()
	req.RecursionDesired = true
	req.Question = []dns.Question{
		{Name: domain + ".", Qtype: rrType, Qclass: class},
	}

	start := time.Now()
	_, err = u.Exchange(&req)
	d := time.Since(start)

	if err != nil {
		return MaxDuration, err
	}
	return d, nil

	// os.Stdout.WriteString("dnslookup result:\n")
	// os.Stdout.WriteString(reply.String() + "\n")
}

func min[T constraints.Ordered](a, b T) T {
	if a < b {
		return a
	}
	return b
}

func max[T constraints.Ordered](a, b T) T {
	if a < b {
		return b
	}
	return a
}
