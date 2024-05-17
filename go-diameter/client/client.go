package main

import (
	"errors"
	"flag"
	"log"
	"math/rand"
	"net"
	"strconv"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"github.com/fiorix/go-diameter/v4/diam/dict"
	"github.com/fiorix/go-diameter/v4/diam/sm"
	"github.com/fiorix/go-diameter/v4/diam/sm/smpeer"
)

func main() {
	addr := flag.String("addr", "localhost:3868", "address in form of ip:port to connect to")
	ssl := flag.Bool("ssl", false, "connect to server using tls")
	host := flag.String("diam_host", "client", "diameter identity host")
	realm := flag.String("diam_realm", "go-diameter", "diameter identity realm")
	certFile := flag.String("cert_file", "", "tls client certificate file (optional)")
	keyFile := flag.String("key_file", "", "tls client key file (optional)")
	bench := flag.Bool("bench", false, "benchmark the server by sending ACR messages")
	benchCli := flag.Int("bench_clients", 1, "number of client connections")
	benchMsgs := flag.Int("bench_msgs", 1000, "number of ACR messages to send")
	networkType := flag.String("network_type", "tcp", "protocol type tcp/sctp")

	flag.Parse()
	if len(*addr) == 0 {
		flag.Usage()
	}

	cfg := &sm.Settings{
		OriginHost:       datatype.DiameterIdentity(*host),
		OriginRealm:      datatype.DiameterIdentity(*realm),
		VendorID:         13,
		ProductName:      "go-diameter",
		OriginStateID:    datatype.Unsigned32(time.Now().Unix()),
		FirmwareRevision: 1,
		HostIPAddresses: []datatype.Address{
			datatype.Address(net.ParseIP("127.0.0.1")),
		},
	}

	// Create the state machine (it's a diam.ServeMux) and client.
	mux := sm.New(cfg)

	cli := &sm.Client{
		Dict:               dict.Default,
		Handler:            mux,
		MaxRetransmits:     3,
		RetransmitInterval: time.Second,
		EnableWatchdog:     true,
		WatchdogInterval:   5 * time.Second,
		AuthApplicationID: []*diam.AVP{
			// Advertise support for credit control application
			diam.NewAVP(avp.AuthApplicationID, avp.Mbit, 0, datatype.Unsigned32(4)), // RFC 4006
		},
	}

	// Set message handlers.
	done := make(chan struct{}, 1000)
	mux.Handle("CCA", handleCCA(done))
	mux.Handle("ACA", handleACA(done))

	// Print error reports.
	go printErrors(mux.ErrorReports())

	connect := func() (diam.Conn, error) {
		return dial(cli, *addr, *certFile, *keyFile, *ssl, *networkType)
	}

	if *bench {
		cli.EnableWatchdog = false
		benchmark(connect, cfg, *benchCli, *benchMsgs, done)
		return
	}

	c, err := connect()
	if err != nil {
		log.Fatal(err)
	}
	err = sendCCR(c, cfg)
	if err != nil {
		log.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		log.Fatal("timeout: no CCA answer received")
	}
	return
}

func printErrors(ec <-chan *diam.ErrorReport) {
	for err := range ec {
		log.Println(err)
	}
}

func dial(cli *sm.Client, addr, cert, key string, ssl bool, networkType string) (diam.Conn, error) {
	if ssl {
		return cli.DialNetworkTLS(networkType, addr, cert, key, nil)
	}
	return cli.DialNetwork(networkType, addr)
}

func sendCCR(c diam.Conn, cfg *sm.Settings) error {
	// Get this client's metadata from the connection object,
	// which is set by the state machine after the handshake.
	// It contains the peer's Origin-Host and Realm from the
	// CER/CEA handshake. We use it to populate the AVPs below.
	meta, ok := smpeer.FromContext(c.Context())
	if !ok {
		return errors.New("peer metadata unavailable")
	}
	sid := "session;" + strconv.Itoa(int(rand.Uint32()))
	m := diam.NewRequest(272, 4, nil)
	m.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String(sid))
	m.NewAVP(avp.OriginHost, avp.Mbit, 0, cfg.OriginHost)
	m.NewAVP(avp.OriginRealm, avp.Mbit, 0, cfg.OriginRealm)
	m.NewAVP(avp.DestinationRealm, avp.Mbit, 0, meta.OriginRealm)
	m.NewAVP(avp.DestinationHost, avp.Mbit, 0, meta.OriginHost)
	m.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String("foobar"))
	log.Printf("Sending CCR to %s\n%s", c.RemoteAddr(), m)
	_, err := m.WriteTo(c)
	return err
}

func handleCCA(done chan struct{}) diam.HandlerFunc {
	return func(c diam.Conn, m *diam.Message) {
		log.Printf("Received CCA from %s\n%s", c.RemoteAddr(), m)
		close(done)
	}
}

func handleACA(done chan struct{}) diam.HandlerFunc {
	ok := struct{}{}
	return func(c diam.Conn, m *diam.Message) {
		done <- ok
	}
}

type dialFunc func() (diam.Conn, error)

func benchmark(df dialFunc, cfg *sm.Settings, ncli, msgs int, done chan struct{}) {
	var err error
	c := make([]diam.Conn, ncli)
	log.Println("Connecting", ncli, "clients...")
	for i := 0; i < ncli; i++ {
		c[i], err = df() // Dial and do CER/CEA handshake.
		if err != nil {
			log.Fatal(err)
		}
		defer c[i].Close()
	}
	log.Println("Done. Sending messages...")
	start := time.Now()
	for _, cli := range c {
		go sendACR(cli, cfg, msgs)
	}
	count := 0
	total := ncli * msgs
wait:
	for {
		select {
		case <-done:
			count++
			if count == total {
				break wait
			}
		case <-time.After(time.Second):
			log.Fatal("Timeout waiting for messages.")
		}
	}
	elapsed := time.Since(start)
	total = total * 2 // req+resp
	log.Printf("%d messages in %s: %d/s", total, elapsed,
		int(float64(total)/elapsed.Seconds()))
}

var eventRecord = datatype.Unsigned32(1) // RFC 6733: EVENT_RECORD 1

func sendACR(c diam.Conn, cfg *sm.Settings, n int) {
	// Get this client's metadata from the connection object,
	// which is set by the state machine after the handshake.
	// It contains the peer's Origin-Host and Realm from the
	// CER/CEA handshake. We use it to populate the AVPs below.
	meta, ok := smpeer.FromContext(c.Context())
	if !ok {
		log.Fatal("Client connection does not contain metadata")
	}
	var err error
	var m *diam.Message
	for i := 0; i < n; i++ {
		m = diam.NewRequest(diam.Accounting, 0, c.Dictionary())
		m.NewAVP(avp.SessionID, avp.Mbit, 0,
			datatype.UTF8String(strconv.Itoa(i)))
		m.NewAVP(avp.OriginHost, avp.Mbit, 0, cfg.OriginHost)
		m.NewAVP(avp.OriginRealm, avp.Mbit, 0, cfg.OriginRealm)
		m.NewAVP(avp.DestinationRealm, avp.Mbit, 0, meta.OriginRealm)
		m.NewAVP(avp.AccountingRecordType, avp.Mbit, 0, eventRecord)
		m.NewAVP(avp.AccountingRecordNumber, avp.Mbit, 0,
			datatype.Unsigned32(i))
		m.NewAVP(avp.DestinationHost, avp.Mbit, 0, meta.OriginHost)
		if _, err = m.WriteTo(c); err != nil {
			log.Fatal(err)
		}
	}
}
