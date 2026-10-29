package zeroconf

import (
	"context"
	"log"
	"net"
	"testing"
	"time"

	"github.com/pkg/errors"
)

var (
	mdnsName    = "test--xxxxxxxxxxxx"
	mdnsService = "_test--xxxx._tcp"
	mdnsSubtype = "_test--xxxx._tcp,_fancy"
	mdnsDomain  = "local."
	mdnsPort    = 8888
)

func startMDNS(ctx context.Context, port int, name, service, domain string) {
	// 5353 is default mdns port
	server, err := Register(name, service, domain, port, []string{"txtv=0", "lo=1", "la=2"}, nil)
	if err != nil {
		panic(errors.Wrap(err, "while registering mdns service"))
	}
	defer server.Shutdown()
	log.Printf("Published service: %s, type: %s, domain: %s", name, service, domain)

	<-ctx.Done()

	log.Printf("Shutting down.")

}

func TestBasic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go startMDNS(ctx, mdnsPort, mdnsName, mdnsService, mdnsDomain)

	time.Sleep(time.Second)

	resolver, err := NewResolver(nil)
	if err != nil {
		t.Fatalf("Expected create resolver success, but got %v", err)
	}
	entries := make(chan *ServiceEntry, 100)
	if err := resolver.Browse(ctx, mdnsService, mdnsDomain, entries); err != nil {
		t.Fatalf("Expected browse success, but got %v", err)
	}
	<-ctx.Done()

	if len(entries) != 1 {
		t.Fatalf("Expected number of service entries is 1, but got %d", len(entries))
	}
	result := <-entries
	if result.Domain != mdnsDomain {
		t.Fatalf("Expected domain is %s, but got %s", mdnsDomain, result.Domain)
	}
	if result.Service != mdnsService {
		t.Fatalf("Expected service is %s, but got %s", mdnsService, result.Service)
	}
	if result.Instance != mdnsName {
		t.Fatalf("Expected instance is %s, but got %s", mdnsName, result.Instance)
	}
	if result.Port != mdnsPort {
		t.Fatalf("Expected port is %d, but got %d", mdnsPort, result.Port)
	}
}

func TestNoRegister(t *testing.T) {
	resolver, err := NewResolver(nil)
	if err != nil {
		t.Fatalf("Expected create resolver success, but got %v", err)
	}

	// before register, mdns resolve shuold not have any entry
	entries := make(chan *ServiceEntry)
	go func(results <-chan *ServiceEntry) {
		s := <-results
		if s != nil {
			t.Errorf("Expected empty service entries but got %v", *s)
		}
	}(entries)

	// A type nobody registers keeps this independent of the entries an earlier
	// test leaves behind in the multicast cache.
	unusedService := "_test--unused._tcp"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := resolver.Browse(ctx, unusedService, "local.", entries); err != nil {
		t.Fatalf("Expected browse success, but got %v", err)
	}
	<-ctx.Done()
	cancel()
}

func TestSubtype(t *testing.T) {
	t.Run("browse with subtype", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		go startMDNS(ctx, mdnsPort, mdnsName, mdnsSubtype, mdnsDomain)

		time.Sleep(time.Second)

		resolver, err := NewResolver(nil)
		if err != nil {
			t.Fatalf("Expected create resolver success, but got %v", err)
		}
		entries := make(chan *ServiceEntry, 100)
		if err := resolver.Browse(ctx, mdnsSubtype, mdnsDomain, entries); err != nil {
			t.Fatalf("Expected browse success, but got %v", err)
		}
		<-ctx.Done()

		if len(entries) != 1 {
			t.Fatalf("Expected number of service entries is 1, but got %d", len(entries))
		}
		result := <-entries
		if result.Domain != mdnsDomain {
			t.Fatalf("Expected domain is %s, but got %s", mdnsDomain, result.Domain)
		}
		if result.Service != mdnsService {
			t.Fatalf("Expected service is %s, but got %s", mdnsService, result.Service)
		}
		if result.Instance != mdnsName {
			t.Fatalf("Expected instance is %s, but got %s", mdnsName, result.Instance)
		}
		if result.Port != mdnsPort {
			t.Fatalf("Expected port is %d, but got %d", mdnsPort, result.Port)
		}
	})

	t.Run("browse without subtype", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		go startMDNS(ctx, mdnsPort, mdnsName, mdnsSubtype, mdnsDomain)

		time.Sleep(time.Second)

		resolver, err := NewResolver(nil)
		if err != nil {
			t.Fatalf("Expected create resolver success, but got %v", err)
		}
		entries := make(chan *ServiceEntry, 100)
		if err := resolver.Browse(ctx, mdnsService, mdnsDomain, entries); err != nil {
			t.Fatalf("Expected browse success, but got %v", err)
		}
		<-ctx.Done()

		if len(entries) != 1 {
			t.Fatalf("Expected number of service entries is 1, but got %d", len(entries))
		}
		result := <-entries
		if result.Domain != mdnsDomain {
			t.Fatalf("Expected domain is %s, but got %s", mdnsDomain, result.Domain)
		}
		if result.Service != mdnsService {
			t.Fatalf("Expected service is %s, but got %s", mdnsService, result.Service)
		}
		if result.Instance != mdnsName {
			t.Fatalf("Expected instance is %s, but got %s", mdnsName, result.Instance)
		}
		if result.Port != mdnsPort {
			t.Fatalf("Expected port is %d, but got %d", mdnsPort, result.Port)
		}
	})
}

func TestLoopback(t *testing.T) {
	for _, tc := range []struct {
		name string
		ip   IPType
	}{{"IPv4", IPv4}, {"IPv6", IPv6}} {
		t.Run(tc.name, func(t *testing.T) { testLoopback(t, tc.ip) })
	}
}

func testLoopback(t *testing.T, ip IPType) {
	var lo []net.Interface
	ifaces, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 && iface.Flags&net.FlagUp != 0 {
			lo = append(lo, iface)
		}
	}
	if len(lo) == 0 {
		t.Skip("no loopback interface is up")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server, err := Register(mdnsName, mdnsService, mdnsDomain, mdnsPort, []string{"txtv=0"}, lo)
	if err != nil {
		t.Fatalf("error while registering mdns service: %s", err)
	}
	t.Cleanup(server.Shutdown)

	entries := make(chan *ServiceEntry, 100)
	resolver, err := NewResolver(SelectIfaces(lo), SelectIPTraffic(ip))
	if err != nil {
		t.Fatalf("Expected resolver success, but got %v", err)
	}
	if err := resolver.Browse(ctx, mdnsService, mdnsDomain, entries); err != nil {
		t.Fatalf("Expected browse success, but got %v", err)
	}
	<-ctx.Done()

	if len(entries) != 1 {
		t.Fatalf("Expected number of service entries on the loopback interface is 1, but got %d", len(entries))
	}
	result := <-entries
	if result.Instance != mdnsName {
		t.Fatalf("Expected instance is %s, but got %s", mdnsName, result.Instance)
	}
}
