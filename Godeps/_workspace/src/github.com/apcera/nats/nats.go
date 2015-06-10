// Copyright 2012-2015 Apcera Inc. All rights reserved.

// A Go client for the NATS messaging system (https://nats.io).
package nats

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	mrand "math/rand"
)

const (
	Version              = "1.0.8"
	DefaultURL           = "nats://localhost:4222"
	DefaultPort          = 4222
	DefaultMaxReconnect  = 60
	DefaultReconnectWait = 2 * time.Second
	DefaultTimeout       = 2 * time.Second
	DefaultPingInterval  = 2 * time.Minute
	DefaultMaxPingOut    = 2
	DefaultMaxChanLen    = 65536
	RequestChanLen       = 4
)

// For detection and proper handling of a Stale Connection
const STALE_CONNECTION = "Stale Connection"

var (
	ErrConnectionClosed   = errors.New("nats: Connection Closed")
	ErrSecureConnRequired = errors.New("nats: Secure connection required")
	ErrSecureConnWanted   = errors.New("nats: Secure connection not available")
	ErrBadSubscription    = errors.New("nats: Invalid Subscription")
	ErrSlowConsumer       = errors.New("nats: Slow Consumer, messages dropped")
	ErrTimeout            = errors.New("nats: Timeout")
	ErrBadTimeout         = errors.New("nats: Timeout Invalid")
	ErrAuthorization      = errors.New("nats: Authorization Failed")
	ErrNoServers          = errors.New("nats: No servers available for connection")
	ErrJsonParse          = errors.New("nats: Connect message, json parse err")
	ErrChanArg            = errors.New("nats: Argument needs to be a channel type")
	ErrStaleConnection    = errors.New("nats: " + STALE_CONNECTION)
)

var DefaultOptions = Options{
	AllowReconnect: true,
	MaxReconnect:   DefaultMaxReconnect,
	ReconnectWait:  DefaultReconnectWait,
	Timeout:        DefaultTimeout,
	PingInterval:   DefaultPingInterval,
	MaxPingsOut:    DefaultMaxPingOut,
	SubChanLen:     DefaultMaxChanLen,
}

type Status int

const (
	DISCONNECTED = Status(iota)
	CONNECTED
	CLOSED
	RECONNECTING
)

// ConnHandlers are used for asynchronous events such as
// disconnected and closed connections.
type ConnHandler func(*Conn)

// ErrHandlers are used to process asynchronous errors encountered
// while processing inbound messages.
type ErrHandler func(*Conn, *Subscription, error)

// Options can be used to create a customized Connection.
type Options struct {
	Url            string
	Servers        []string
	NoRandomize    bool
	Name           string
	Verbose        bool
	Pedantic       bool
	Secure         bool
	AllowReconnect bool
	MaxReconnect   int
	ReconnectWait  time.Duration
	Timeout        time.Duration
	ClosedCB       ConnHandler
	DisconnectedCB ConnHandler
	ReconnectedCB  ConnHandler
	AsyncErrorCB   ErrHandler

	PingInterval time.Duration // disabled if 0 or negative
	MaxPingsOut  int

	// The size of the buffered channel used between the socket
	// Go routine and the message delivery or sync subscription.
	SubChanLen int
}

const (
	// Scratch storage for assembling protocol headers
	scratchSize = 512

	// The size of the bufio reader/writer on top of the socket.
	defaultBufSize = 32768

	// The size of the bufio while we are reconnecting
	defaultPendingSize = 1024 * 1024

	// The buffered size of the flush "kick" channel
	flushChanSize = 1024

	// Default server pool size
	srvPoolSize = 4
)

// A Conn represents a bare connection to a nats-server. It will send and receive
// []byte payloads.
type Conn struct {
	Statistics
	mu      sync.Mutex
	Opts    Options
	wg      sync.WaitGroup
	url     *url.URL
	conn    net.Conn
	srvPool []*srv
	bw      *bufio.Writer
	pending *bytes.Buffer
	fch     chan bool
	info    serverInfo
	ssid    int64
	subs    map[int64]*Subscription
	mch     chan *Msg
	pongs   []chan bool
	scratch [scratchSize]byte
	status  Status
	err     error
	ps      *parseState
	ptmr    *time.Timer
	pout    int
}

// A Subscription represents interest in a given subject.
type Subscription struct {
	mu  sync.Mutex
	sid int64

	// Subject that represents this subscription. This can be different
	// than the received subject inside a Msg if this is a wildcard.
	Subject string

	// Optional queue group name. If present, all subscriptions with the
	// same name will form a distributed queue, and each message will
	// only be processed by one member of the group.
	Queue string

	msgs      uint64
	delivered uint64
	bytes     uint64
	max       uint64
	conn      *Conn
	mcb       MsgHandler
	mch       chan *Msg
	sc        bool
}

// Msg is a structure used by Subscribers and PublishMsg().
type Msg struct {
	Subject string
	Reply   string
	Data    []byte
	Sub     *Subscription
}

// Tracks various stats received and sent on this connection,
// including counts for messages and bytes.
type Statistics struct {
	InMsgs     uint64
	OutMsgs    uint64
	InBytes    uint64
	OutBytes   uint64
	Reconnects uint64
}

// Tracks individual backend servers.
type srv struct {
	url         *url.URL
	didConnect  bool
	reconnects  int
	lastAttempt time.Time
}

type serverInfo struct {
	Id           string `json:"server_id"`
	Host         string `json:"host"`
	Port         uint   `json:"port"`
	Version      string `json:"version"`
	AuthRequired bool   `json:"auth_required"`
	SslRequired  bool   `json:"ssl_required"`
	MaxPayload   int64  `json:"max_payload"`
}

type connectInfo struct {
	Verbose  bool   `json:"verbose"`
	Pedantic bool   `json:"pedantic"`
	User     string `json:"user,omitempty"`
	Pass     string `json:"pass,omitempty"`
	Ssl      bool   `json:"ssl_required"`
	Name     string `json:"name"`
}

// MsgHandler is a callback function that processes messages delivered to
// asynchronous subscribers.
type MsgHandler func(msg *Msg)

// Connect will attempt to connect to the NATS server.
// The url can contain username/password semantics.
func Connect(url string) (*Conn, error) {
	opts := DefaultOptions
	opts.Url = url
	return opts.Connect()
}

// SecureConnect will attempt to connect to the NATS server using TLS.
// The url can contain username/password semantics.
func SecureConnect(url string) (*Conn, error) {
	opts := DefaultOptions
	opts.Url = url
	opts.Secure = true
	return opts.Connect()
}

// Connect will attempt to connect to a NATS server with multiple options.
func (o Options) Connect() (*Conn, error) {
	nc := &Conn{Opts: o}
	if nc.Opts.MaxPingsOut == 0 {
		nc.Opts.MaxPingsOut = DefaultMaxPingOut
	}
	// Allow old default for channel length to work correctly.
	if nc.Opts.SubChanLen == 0 {
		nc.Opts.SubChanLen = DefaultMaxChanLen
	}
	if err := nc.setupServerPool(); err != nil {
		return nil, err
	}
	if err := nc.connect(); err != nil {
		return nil, err
	}
	return nc, nil
}

const (
	_CRLF_  = "\r\n"
	_EMPTY_ = ""
	_SPC_   = " "
	_PUB_P_ = "PUB "
)

const (
	_OK_OP_   = "+OK"
	_ERR_OP_  = "-ERR"
	_MSG_OP_  = "MSG"
	_PING_OP_ = "PING"
	_PONG_OP_ = "PONG"
	_INFO_OP_ = "INFO"
)

const (
	conProto   = "CONNECT %s" + _CRLF_
	pingProto  = "PING" + _CRLF_
	pongProto  = "PONG" + _CRLF_
	pubProto   = "PUB %s %s %d" + _CRLF_
	subProto   = "SUB %s %s %d" + _CRLF_
	unsubProto = "UNSUB %d %s" + _CRLF_
)

// Return bool indicating if we have more servers to try to establish a connection.
func (nc *Conn) serversAvailable() bool {
	for _, s := range nc.srvPool {
		if s != nil {
			return true
		}
	}
	return false
}

func (nc *Conn) debugPool(str string) {
	_, cur := nc.currentServer()
	fmt.Printf("%s\n", str)
	for i, s := range nc.srvPool {
		if s == cur {
			fmt.Printf("\t*%d: %v\n", i+1, s.url)
		} else {
			fmt.Printf("\t%d: %v\n", i+1, s.url)
		}
	}
}

// Return the currently selected server
func (nc *Conn) currentServer() (int, *srv) {
	for i, s := range nc.srvPool {
		if s == nil {
			continue
		}
		if s.url == nc.url {
			return i, s
		}
	}
	return -1, nil
}

// Pop the current server and put onto the end of the list. Select head of list as long
// as number of reconnect attempts under MaxReconnect.
func (nc *Conn) selectNextServer() (*srv, error) {
	i, s := nc.currentServer()
	if i < 0 {
		return nil, ErrNoServers
	}
	sp := nc.srvPool
	num := len(sp)
	copy(sp[i:num-1], sp[i+1:num])
	max_reconnect := nc.Opts.MaxReconnect
	if max_reconnect < 0 || s.reconnects < max_reconnect {
		nc.srvPool[num-1] = s
	} else {
		nc.srvPool = sp[0 : num-1]
	}
	if len(nc.srvPool) <= 0 {
		nc.url = nil
		return nil, ErrNoServers
	}
	nc.url = nc.srvPool[0].url
	return nc.srvPool[0], nil
}

// Will assign the correct server to the nc.Url
func (nc *Conn) pickServer() error {
	nc.url = nil
	if len(nc.srvPool) <= 0 {
		return ErrNoServers
	}
	for _, s := range nc.srvPool {
		if s != nil {
			nc.url = s.url
			return nil
		}
	}
	return ErrNoServers
}

// Create the server pool using the options given.
// We will place a Url option first, followed by any
// Server Options. We will randomize the server pool unlesss
// the NoRandomize flag is set.
func (nc *Conn) setupServerPool() error {
	nc.srvPool = make([]*srv, 0, srvPoolSize)
	if nc.Opts.Url != _EMPTY_ {
		u, err := url.Parse(nc.Opts.Url)
		if err != nil {
			return err
		}
		s := &srv{url: u}
		nc.srvPool = append(nc.srvPool, s)
	}

	var srvrs []string
	source := mrand.NewSource(time.Now().UnixNano())
	r := mrand.New(source)

	if nc.Opts.NoRandomize {
		srvrs = nc.Opts.Servers
	} else {
		in := r.Perm(len(nc.Opts.Servers))
		for _, i := range in {
			srvrs = append(srvrs, nc.Opts.Servers[i])
		}
	}
	for _, urlString := range srvrs {
		u, err := url.Parse(urlString)
		if err != nil {
			return err
		}
		s := &srv{url: u}
		nc.srvPool = append(nc.srvPool, s)
	}
	return nc.pickServer()
}

// createConn will connect to the server and wrap the appropriate
// bufio structures. It will do the right thing when an existing
// connection is in place.
func (nc *Conn) createConn() error {
	if nc.Opts.Timeout < 0 {
		return ErrBadTimeout
	}
	if _, cur := nc.currentServer(); cur == nil {
		return ErrNoServers
	} else {
		cur.lastAttempt = time.Now()
	}
	nc.conn, nc.err = net.DialTimeout("tcp", nc.url.Host, nc.Opts.Timeout)
	if nc.err != nil {
		return nc.err
	}

	// No clue why, but this stalls and kills performance on Mac (Mavericks).
	// https://code.google.com/p/go/issues/detail?id=6930
	//if ip, ok := nc.conn.(*net.TCPConn); ok {
	//	ip.SetReadBuffer(defaultBufSize)
	//}

	if nc.pending != nil && nc.bw != nil {
		// Move to pending buffer.
		nc.bw.Flush()
	}
	nc.bw = bufio.NewWriterSize(nc.conn, defaultBufSize)
	return nil
}

// makeSecureConn will wrap an existing Conn using TLS
func (nc *Conn) makeTLSConn() {
	nc.conn = tls.Client(nc.conn, &tls.Config{InsecureSkipVerify: true})
	nc.bw = bufio.NewWriterSize(nc.conn, defaultBufSize)
}

// waitForExits will wait for all socket watcher Go routines to
// be shutdown before proceeding.
func (nc *Conn) waitForExits() {
	// Kick old flusher forcefully.
	nc.fch <- true
	// Wait for any previous go routines.
	nc.wg.Wait()
}

// spinUpSocketWatchers will launch the Go routines responsible for
// reading and writing to the socket. This will be launched via a
// go routine itself to release any locks that may be held.
// We also use a WaitGroup to make sure we only start them on a
// reconnect when the previous ones have exited.
func (nc *Conn) spinUpSocketWatchers() {
	// Make sure everything has exited.
	nc.waitForExits()

	// We will wait on both going forward.
	nc.wg.Add(2)

	// Spin up the readLoop and the socket flusher.
	go nc.readLoop()
	go nc.flusher()

	nc.mu.Lock()
	nc.pout = 0

	if nc.Opts.PingInterval > 0 {
		if nc.ptmr == nil {
			nc.ptmr = time.AfterFunc(nc.Opts.PingInterval, nc.processPingTimer)
		} else {
			nc.ptmr.Reset(nc.Opts.PingInterval)
		}
	}
	nc.mu.Unlock()
}

// Report the connected server's Url
func (nc *Conn) ConnectedUrl() string {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	if nc.status != CONNECTED {
		return _EMPTY_
	}
	return nc.url.String()
}

// Report the connected server's Id
func (nc *Conn) ConnectedServerId() string {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	if nc.status != CONNECTED {
		return _EMPTY_
	}
	return nc.info.Id
}

// Low level setup for structs, etc
func (nc *Conn) setup() {
	nc.subs = make(map[int64]*Subscription)
	nc.pongs = make([]chan bool, 0, 8)

	nc.fch = make(chan bool, flushChanSize)

	// Setup scratch outbound buffer for PUB
	pub := nc.scratch[:len(_PUB_P_)]
	copy(pub, _PUB_P_)
}

// Process a connected connection and initialize properly.
// The lock should not be held entering this function.
func (nc *Conn) processConnectInit() error {
	nc.mu.Lock()
	nc.setup()

	// Set our status to connected.
	nc.status = CONNECTED

	// Make sure to process the INFO inline here.
	if nc.err = nc.processExpectedInfo(); nc.err != nil {
		nc.mu.Unlock()
		return nc.err
	}
	nc.mu.Unlock()

	// We need these to process the sendConnect.
	go nc.spinUpSocketWatchers()

	return nc.sendConnect()
}

// Main connect function. Will connect to the nats-server
func (nc *Conn) connect() error {
	// Create actual socket connection
	// For first connect we walk all servers in the pool and try
	// to connect immediately.
	nc.mu.Lock()
	for i := range nc.srvPool {
		nc.url = nc.srvPool[i].url
		if err := nc.createConn(); err == nil {
			// Release the lock, processConnectInit has to do its own locking.
			nc.mu.Unlock()
			err = nc.processConnectInit()
			nc.mu.Lock()

			if err == nil {
				nc.srvPool[i].didConnect = true
				nc.srvPool[i].reconnects = 0
				break
			} else {
				nc.err = err
				nc.mu.Unlock()
				nc.close(DISCONNECTED, false)
				nc.mu.Lock()
				nc.url = nil
			}
		} else {
			// Cancel out default connection refused, will trigger the
			// No servers error conditional
			if matched, _ := regexp.Match(`connection refused`, []byte(err.Error())); matched {
				nc.err = nil
			}
		}
	}
	defer nc.mu.Unlock()

	if nc.err == nil && nc.status != CONNECTED {
		nc.err = ErrNoServers
	}
	return nc.err
}

// This will check to see if the connection should be
// secure. This can be dictated from either end and should
// only be called after the INIT protocol has been received.
func (nc *Conn) checkForSecure() error {
	// Check to see if we need to engage TLS
	o := nc.Opts

	// Check for mismatch in setups
	if o.Secure && !nc.info.SslRequired {
		return ErrSecureConnWanted
	} else if nc.info.SslRequired && !o.Secure {
		return ErrSecureConnRequired
	}

	// Need to rewrap with bufio
	if o.Secure {
		nc.makeTLSConn()
	}
	return nil
}

// processExpectedInfo will look for the expected first INFO message
// sent when a connection is established. The lock should be held entering.
func (nc *Conn) processExpectedInfo() error {
	nc.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	defer nc.conn.SetReadDeadline(time.Time{})

	c := &control{}
	if err := nc.readOp(c); err != nil {
		nc.mu.Unlock()
		nc.processOpErr(err)
		nc.mu.Lock()
		return err
	}
	// The nats protocol should send INFO first always.
	if c.op != _INFO_OP_ {
		nc.mu.Unlock()
		err := errors.New("nats: Protocol exception, INFO not received")
		nc.processOpErr(err)
		nc.mu.Lock()
		return err
	}
	nc.processInfo(c.args)
	return nc.checkForSecure()
}

// Sends a protocol control message by queueing into the bufio writer
// and kicking the flush Go routine.  These writes are protected.
func (nc *Conn) sendProto(proto string) {
	nc.mu.Lock()
	nc.bw.WriteString(proto)
	nc.kickFlusher()
	nc.mu.Unlock()
}

// Generate a connect protocol message, issuing user/password if
// applicable. The lock is assumed to be held upon entering.
func (nc *Conn) connectProto() (string, error) {
	o := nc.Opts
	var user, pass string
	u := nc.url.User
	if u != nil {
		user = u.Username()
		pass, _ = u.Password()
	}
	cinfo := connectInfo{o.Verbose, o.Pedantic, user, pass, o.Secure, o.Name}
	b, err := json.Marshal(cinfo)
	if err != nil {
		nc.err = ErrJsonParse
		return _EMPTY_, nc.err
	}
	return fmt.Sprintf(conProto, b), nil
}

// Send a connect protocol message to the server, issuing user/password if
// applicable. Will wait for a flush to return from the server for error
// processing. The lock should not be held entering this function.
func (nc *Conn) sendConnect() error {

	nc.mu.Lock()
	cProto, err := nc.connectProto()
	if err != nil {
		nc.mu.Unlock()
		return err
	}
	nc.mu.Unlock()

	nc.sendProto(cProto)

	if err := nc.FlushTimeout(DefaultTimeout); err != nil {
		nc.err = err
		return err
	}

	nc.mu.Lock()
	defer nc.mu.Unlock()

	if nc.isClosed() {
		return nc.err
	}
	nc.status = CONNECTED
	return nil
}

// A control protocol line.
type control struct {
	op, args string
}

// Read a control line and process the intended op.
func (nc *Conn) readOp(c *control) error {
	if nc.isClosed() {
		return ErrConnectionClosed
	}
	br := bufio.NewReaderSize(nc.conn, defaultBufSize)
	b, pre, err := br.ReadLine()
	if err != nil {
		return err
	}
	if pre {
		// FIXME: Be more specific here?
		return errors.New("nats: Line too long")
	}
	// Do straight move to string rep.
	line := *(*string)(unsafe.Pointer(&b))
	parseControl(line, c)
	return nil
}

// Parse a control line from the server.
func parseControl(line string, c *control) {
	toks := strings.SplitN(line, _SPC_, 2)
	if len(toks) == 1 {
		c.op = strings.TrimSpace(toks[0])
		c.args = _EMPTY_
	} else if len(toks) == 2 {
		c.op, c.args = strings.TrimSpace(toks[0]), strings.TrimSpace(toks[1])
	} else {
		c.op = _EMPTY_
	}
}

func (nc *Conn) processDisconnect() {
	nc.status = DISCONNECTED
	if nc.err != nil {
		return
	}
	if nc.info.SslRequired {
		nc.err = ErrSecureConnRequired
	} else {
		nc.err = ErrConnectionClosed
	}
}

// This will process a disconnect when reconnect is allowed.
// The lock should not be held on entering this function.
func (nc *Conn) processReconnect() {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	if !nc.isClosed() {
		// If we are already in the proper state, just return.
		if nc.isReconnecting() {
			return
		}
		nc.status = RECONNECTING
		if nc.ptmr != nil {
			nc.ptmr.Stop()
		}
		if nc.conn != nil {
			nc.bw.Flush()
			nc.conn.Close()
			nc.conn = nil
		}
		go nc.doReconnect()
	}
}

// flushReconnectPending will push the pending items that were
// gathered while we were in a RECONNECTING state to the socket.
func (nc *Conn) flushReconnectPendingItems() {
	if nc.pending == nil {
		return
	}
	if nc.pending.Len() > 0 {
		nc.bw.Write(nc.pending.Bytes())
	}
	nc.pending = nil
}

// Try to reconnect using the option parameters.
// This function assumes we are allowed to reconnect.
func (nc *Conn) doReconnect() {
	// We want to make sure we have the other watchers shutdown properly
	// here before we proceed past this point.
	nc.waitForExits()

	// FIXME(dlc) - We have an issue here if we have
	// outstanding flush points (pongs) and they were not
	// sent out, but are still in the pipe.

	// Hold the lock manually and release where needed below,
	// can't do defer here.
	nc.mu.Lock()

	// Create a new pending buffer to underpin the bufio Writer while
	// we are reconnecting.
	nc.pending = &bytes.Buffer{}
	nc.bw = bufio.NewWriterSize(nc.pending, defaultPendingSize)

	// Clear any errors.
	nc.err = nil

	// Perform appropriate callback if needed for a disconnect.
	dcb := nc.Opts.DisconnectedCB
	if dcb != nil {
		nc.mu.Unlock()
		dcb(nc)
		nc.mu.Lock()
	}

	for len(nc.srvPool) > 0 {
		cur, err := nc.selectNextServer()
		if err != nil {
			nc.err = err
			break
		}

		// Sleep appropriate amount of time before the
		// connection attempt if connecting to same server
		// we just got disconnected from..
		if time.Since(cur.lastAttempt) < nc.Opts.ReconnectWait {
			sleepTime := nc.Opts.ReconnectWait - time.Since(cur.lastAttempt)
			nc.mu.Unlock()
			time.Sleep(sleepTime)
			nc.mu.Lock()
		}

		// Check if we have been closed first.
		if nc.isClosed() {
			break
		}

		// Mark that we tried a reconnect
		cur.reconnects += 1

		// Try to create a new connection
		err = nc.createConn()

		// Not yet connected, retry...
		// Continue to hold the lock
		if err != nil {
			nc.err = nil
			continue
		}

		// We are reconnected
		nc.Reconnects += 1

		// Clear out server stats for the server we connected to..
		cur.didConnect = true
		cur.reconnects = 0

		// Process Connect logic
		if nc.err = nc.processExpectedInfo(); nc.err == nil {
			// Send our connect info as normal
			cProto, err := nc.connectProto()
			if err != nil {
				continue
			}

			// Set our status to connected.
			nc.status = CONNECTED

			nc.bw.WriteString(cProto)
			// Send existing subscription state
			nc.resendSubscriptions()
			// Now send off and clear pending buffer
			nc.flushReconnectPendingItems()

			// Spin up socket watchers again
			go nc.spinUpSocketWatchers()
		} else {
			nc.status = RECONNECTING
			continue
		}

		// snapshot the reconnect callback while lock is held.
		rcb := nc.Opts.ReconnectedCB

		// Release lock here, we will return below.
		nc.mu.Unlock()

		// Make sure to flush everything
		nc.Flush()

		// Call reconnectedCB if appropriate. We are already in a
		// separate Go routine here, so ok to call direct.
		if rcb != nil {
			rcb(nc)
		}
		return
	}

	// Call into close.. We have no servers left..
	if nc.err == nil {
		nc.err = ErrNoServers
	}
	nc.mu.Unlock()
	nc.Close()
}

// processOpErr handles errors from reading or parsing the protocol.
// The lock should not be held entering this function.
func (nc *Conn) processOpErr(err error) {
	nc.mu.Lock()
	if nc.isClosed() || nc.isReconnecting() {
		nc.mu.Unlock()
		return
	}
	allowReconnect := nc.Opts.AllowReconnect
	nc.mu.Unlock()

	if allowReconnect {
		nc.processReconnect()
	} else {
		nc.mu.Lock()
		nc.processDisconnect()
		nc.err = err
		nc.mu.Unlock()
		nc.Close()
	}
}

// readLoop() will sit on the socket reading and processing the
// protocol from the server. It will dispatch appropriately based
// on the op type.
func (nc *Conn) readLoop() {
	// Release the wait group on exit
	defer nc.wg.Done()

	// Create a parseState if needed.
	nc.mu.Lock()
	if nc.ps == nil {
		nc.ps = &parseState{}
	}
	nc.mu.Unlock()

	// Stack based buffer.
	b := make([]byte, defaultBufSize)

	for {
		// FIXME(dlc): RWLock here?
		nc.mu.Lock()
		sb := nc.isClosed() || nc.isReconnecting()
		if sb {
			nc.ps = &parseState{}
		}
		conn := nc.conn
		nc.mu.Unlock()

		if sb || conn == nil {
			break
		}

		n, err := conn.Read(b)
		if err != nil {
			nc.processOpErr(err)
			break
		}
		if err := nc.parse(b[:n]); err != nil {
			nc.processOpErr(err)
			break
		}
	}
	// Clear the parseState here..
	nc.mu.Lock()
	nc.ps = nil
	nc.mu.Unlock()
}

// deliverMsgs waits on the delivery channel shared with readLoop and processMsg.
// It is used to deliver messages to asynchronous subscribers.
func (nc *Conn) deliverMsgs(ch chan *Msg) {
	for {
		nc.mu.Lock()
		closed := nc.isClosed()
		nc.mu.Unlock()
		if closed {
			break
		}

		m, ok := <-ch
		if !ok {
			break
		}
		s := m.Sub

		// Capture under locks
		s.mu.Lock()
		conn := s.conn
		mcb := s.mcb
		s.mu.Unlock()

		if conn == nil || mcb == nil {
			continue
		}
		// FIXME: race on compare?
		atomic.AddUint64(&s.delivered, 1)
		if s.max <= 0 || s.delivered <= s.max {
			mcb(m)
		}
	}
}

// processMsg is called by parse and will place the msg on the
// appropriate channel for processing. All subscribers have their
// their own channel. If the channel is full, the connection is
// considered a slow subscriber.
func (nc *Conn) processMsg(msg []byte) {
	// Lock from here on out.
	nc.mu.Lock()

	// Stats
	nc.InMsgs += 1
	nc.InBytes += uint64(len(msg))

	sub := nc.subs[nc.ps.ma.sid]
	if sub == nil {
		nc.mu.Unlock()
		return
	}

	sub.mu.Lock()

	if sub.max > 0 && sub.msgs > sub.max {
		sub.mu.Unlock()
		nc.mu.Unlock()
		return
	}

	// Sub internal stats
	sub.msgs += 1
	sub.bytes += uint64(len(msg))

	// Copy them into string
	subj := string(nc.ps.ma.subject)
	reply := string(nc.ps.ma.reply)

	// FIXME(dlc): Need to copy, should/can do COW?
	newMsg := make([]byte, len(msg))
	copy(newMsg, msg)

	// FIXME(dlc): Should we recycle these containers?
	m := &Msg{Data: newMsg, Subject: subj, Reply: reply, Sub: sub}

	if sub.mch != nil {
		if len(sub.mch) >= nc.Opts.SubChanLen {
			nc.processSlowConsumer(sub)
		} else {
			// Clear always
			sub.sc = false
			sub.mch <- m
		}
	}

	sub.mu.Unlock()
	nc.mu.Unlock()
}

// processSlowConsumer will set SlowConsumer state and fire the
// async error handler if registered.
func (nc *Conn) processSlowConsumer(s *Subscription) {
	nc.err = ErrSlowConsumer
	if nc.Opts.AsyncErrorCB != nil && !s.sc {
		go nc.Opts.AsyncErrorCB(nc, s, nc.err)
	}
	s.sc = true
}

// flusher is a separate Go routine that will process flush requests for the write
// bufio. This allows coalescing of writes to the underlying socket.
func (nc *Conn) flusher() {
	// Release the wait group
	defer nc.wg.Done()

	for {
		if _, ok := <-nc.fch; !ok {
			return
		}
		nc.mu.Lock()

		// Check for closed or reconnecting
		if nc.isClosed() || nc.isReconnecting() {
			nc.mu.Unlock()
			return
		}
		b := nc.bw.Buffered()
		if b > 0 && nc.conn != nil {
			nc.err = nc.bw.Flush()
		}
		nc.mu.Unlock()
	}
}

// processPing will send an immediate pong protocol response to the
// server. The server uses this mechanism to detect dead clients.
func (nc *Conn) processPing() {
	nc.sendProto(pongProto)
}

// processPong is used to process responses to the client's ping
// messages. We use pings for the flush mechanism as well.
func (nc *Conn) processPong() {
	var ch chan bool

	nc.mu.Lock()
	if len(nc.pongs) > 0 {
		ch = nc.pongs[0]
		nc.pongs = nc.pongs[1:]
	}
	nc.pout = 0
	nc.mu.Unlock()
	if ch != nil {
		ch <- true
	}
}

// processOK is a placeholder for processing OK messages.
func (nc *Conn) processOK() {
	// do nothing
}

// processInfo is used to parse the info messages sent
// from the server.
func (nc *Conn) processInfo(info string) {
	if info == _EMPTY_ {
		return
	}
	nc.err = json.Unmarshal([]byte(info), &nc.info)
}

// LastError reports the last error encountered via the Connection.
func (nc *Conn) LastError() error {
	return nc.err
}

// processErr processes any error messages from the server and
// sets the connection's lastError.
func (nc *Conn) processErr(e string) {
	// FIXME(dlc) - process Slow Consumer signals special.
	if e == STALE_CONNECTION {
		nc.processOpErr(ErrStaleConnection)
	} else {
		nc.mu.Lock()
		nc.err = errors.New("nats: " + e)
		nc.mu.Unlock()
		nc.Close()
	}
}

// kickFlusher will send a bool on a channel to kick the
// flush Go routine to flush data to the server.
func (nc *Conn) kickFlusher() {
	if len(nc.fch) == 0 && nc.bw != nil {
		nc.fch <- true
	}
}

// Used for handrolled itoa
const digits = "0123456789"

// publish is the internal function to publish messages to a nats-server.
// Sends a protocol data message by queueing into the bufio writer
// and kicking the flush go routine. These writes should be protected.
func (nc *Conn) publish(subj, reply string, data []byte) error {
	nc.mu.Lock()

	if nc.isClosed() {
		nc.mu.Unlock()
		return ErrConnectionClosed
	}

	if nc.err != nil {
		err := nc.err
		nc.mu.Unlock()
		return err
	}

	msgh := nc.scratch[:len(_PUB_P_)]
	msgh = append(msgh, subj...)
	msgh = append(msgh, ' ')
	if reply != "" {
		msgh = append(msgh, reply...)
		msgh = append(msgh, ' ')
	}

	// We could be smarter here, but simple loop is ok,
	// just avoid strconv in fast path
	// FIXME(dlc) - Find a better way here.
	// msgh = strconv.AppendInt(msgh, int64(len(data)), 10)

	var b [12]byte
	var i = len(b)
	if len(data) > 0 {
		for l := len(data); l > 0; l /= 10 {
			i -= 1
			b[i] = digits[l%10]
		}
	} else {
		i -= 1
		b[i] = digits[0]
	}

	msgh = append(msgh, b[i:]...)
	msgh = append(msgh, _CRLF_...)

	// FIXME, do deadlines here
	if _, nc.err = nc.bw.Write(msgh); nc.err != nil {
		nc.mu.Unlock()
		return nc.err
	}
	if _, nc.err = nc.bw.Write(data); nc.err != nil {
		nc.mu.Unlock()
		return nc.err
	}

	if _, nc.err = nc.bw.WriteString(_CRLF_); nc.err != nil {
		nc.mu.Unlock()
		return nc.err
	}

	nc.OutMsgs += 1
	nc.OutBytes += uint64(len(data))

	nc.kickFlusher()
	nc.mu.Unlock()
	return nil
}

// Publish publishes the data argument to the given subject. The data
// argument is left untouched and needs to be correctly interpreted on
// the receiver.
func (nc *Conn) Publish(subj string, data []byte) error {
	return nc.publish(subj, _EMPTY_, data)
}

// PublishMsg publishes the Msg structure, which includes the
// Subject, an optional Reply and an optional Data field.
func (nc *Conn) PublishMsg(m *Msg) error {
	return nc.publish(m.Subject, m.Reply, m.Data)
}

// PublishRequest will perform a Publish() excpecting a response on the
// reply subject. Use Request() for automatically waiting for a response
// inline.
func (nc *Conn) PublishRequest(subj, reply string, data []byte) error {
	return nc.publish(subj, reply, data)
}

// Request will create an Inbox and perform a Request() call
// with the Inbox reply and return the first reply received.
// This is optimized for the case of multiple responses.
func (nc *Conn) Request(subj string, data []byte, timeout time.Duration) (m *Msg, err error) {
	inbox := NewInbox()
	s, err := nc.subscribe(inbox, _EMPTY_, nil, RequestChanLen)
	if err != nil {
		return nil, err
	}
	s.AutoUnsubscribe(1)
	err = nc.PublishRequest(subj, inbox, data)
	if err == nil {
		m, err = s.NextMsg(timeout)
	}
	s.Unsubscribe()
	return
}

const InboxPrefix = "_INBOX."

// NewInbox will return an inbox string which can be used for directed replies from
// subscribers. These are guaranteed to be unique, but can be shared and subscribed
// to by others.
func NewInbox() string {
	u := make([]byte, 13)
	io.ReadFull(rand.Reader, u)
	return fmt.Sprintf("%s%s", InboxPrefix, hex.EncodeToString(u))
}

// subscribe is the internal subscribe function that indicates interest in a subject.
func (nc *Conn) subscribe(subj, queue string, cb MsgHandler, chanlen int) (*Subscription, error) {
	nc.mu.Lock()
	// ok here, but defer is expensive
	defer nc.kickFlusher()
	defer nc.mu.Unlock()

	if nc.isClosed() {
		return nil, ErrConnectionClosed
	}

	sub := &Subscription{Subject: subj, Queue: queue, mcb: cb, conn: nc}
	sub.mch = make(chan *Msg, chanlen)

	// If we have an async callback, start up a sub specific
	// Go routine to deliver the messages.
	if cb != nil {
		go nc.deliverMsgs(sub.mch)
	}

	sub.sid = atomic.AddInt64(&nc.ssid, 1)
	nc.subs[sub.sid] = sub

	// We will send these for all subs when we reconnect
	// so that we can suppress here.
	if !nc.isReconnecting() {
		nc.bw.WriteString(fmt.Sprintf(subProto, subj, queue, sub.sid))
	}
	return sub, nil
}

// Subscribe will express interest in the given subject. The subject
// can have wildcards (partial:*, full:>). Messages will be delivered
// to the associated MsgHandler. If no MsgHandler is given, the
// subscription is a synchronous subscription and can be polled via
// Subscription.NextMsg().
func (nc *Conn) Subscribe(subj string, cb MsgHandler) (*Subscription, error) {
	return nc.subscribe(subj, _EMPTY_, cb, nc.Opts.SubChanLen)
}

// SubscribeSync is syntactic sugar for Subscribe(subject, nil).
func (nc *Conn) SubscribeSync(subj string) (*Subscription, error) {
	return nc.subscribe(subj, _EMPTY_, nil, nc.Opts.SubChanLen)
}

// QueueSubscribe creates an asynchronous queue subscriber on the given subject.
// All subscribers with the same queue name will form the queue group and
// only one member of the group will be selected to receive any given
// message asynchronously.
func (nc *Conn) QueueSubscribe(subj, queue string, cb MsgHandler) (*Subscription, error) {
	return nc.subscribe(subj, queue, cb, nc.Opts.SubChanLen)
}

// QueueSubscribeSync creates a synchronous queue subscriber on the given
// subject. All subscribers with the same queue name will form the queue
// group and only one member of the group will be selected to receive any
// given message synchronously.
func (nc *Conn) QueueSubscribeSync(subj, queue string) (*Subscription, error) {
	return nc.subscribe(subj, queue, nil, nc.Opts.SubChanLen)
}

// unsubscribe performs the low level unsubscribe to the server.
// Use Subscription.Unsubscribe()
func (nc *Conn) unsubscribe(sub *Subscription, max int) error {
	nc.mu.Lock()
	// ok here, but defer is expensive
	defer nc.kickFlusher()
	defer nc.mu.Unlock()

	if nc.isClosed() {
		return ErrConnectionClosed
	}

	s := nc.subs[sub.sid]
	// Already unsubscribed
	if s == nil {
		return nil
	}

	maxStr := _EMPTY_
	if max > 0 {
		s.max = uint64(max)
		maxStr = strconv.Itoa(max)
	} else {
		delete(nc.subs, s.sid)
		s.mu.Lock()
		if s.mch != nil {
			close(s.mch)
			s.mch = nil
		}
		// Mark as invalid
		s.conn = nil
		s.mu.Unlock()
	}
	// We will send these for all subs when we reconnect
	// so that we can suppress here.
	if !nc.isReconnecting() {
		nc.bw.WriteString(fmt.Sprintf(unsubProto, s.sid, maxStr))
	}
	return nil
}

// IsValid returns a boolean indicating whether the subscription
// is still active. This will return false if the subscription has
// already been closed.
func (s *Subscription) IsValid() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn != nil
}

// Unsubscribe will remove interest in the given subject.
func (s *Subscription) Unsubscribe() error {
	s.mu.Lock()
	conn := s.conn
	s.mu.Unlock()
	if conn == nil {
		return ErrBadSubscription
	}
	return conn.unsubscribe(s, 0)
}

// AutoUnsubscribe will issue an automatic Unsubscribe that is
// processed by the server when max messages have been received.
// This can be useful when sending a request to an unknown number
// of subscribers. Request() uses this functionality.
func (s *Subscription) AutoUnsubscribe(max int) error {
	s.mu.Lock()
	conn := s.conn
	s.mu.Unlock()
	if conn == nil {
		return ErrBadSubscription
	}
	return conn.unsubscribe(s, max)
}

// NextMsg() will return the next message available to a synchronous subscriber
// or block until one is available. A timeout can be used to return when no
// message has been delivered.
func (s *Subscription) NextMsg(timeout time.Duration) (msg *Msg, err error) {
	s.mu.Lock()
	if s.mch == nil {
		s.mu.Unlock()
		return nil, ErrConnectionClosed
	}
	if s.mcb != nil {
		s.mu.Unlock()
		return nil, errors.New("nats: Illegal call on an async Subscription")
	}
	if s.conn == nil {
		s.mu.Unlock()
		return nil, ErrBadSubscription
	}
	if s.sc {
		s.sc = false
		s.mu.Unlock()
		return nil, ErrSlowConsumer
	}

	mch := s.mch
	s.mu.Unlock()

	var ok bool
	t := time.NewTimer(timeout)
	defer t.Stop()

	select {
	case msg, ok = <-mch:
		if !ok {
			return nil, ErrConnectionClosed
		}
		atomic.AddUint64(&s.delivered, 1)
		if s.max > 0 && s.delivered > s.max {
			return nil, errors.New("nats: Max messages delivered")
		}
	case <-t.C:
		return nil, ErrTimeout
	}
	return
}

// FIXME: This is a hack
// removeFlushEntry is needed when we need to discard queued up responses
// for our pings as part of a flush call. This happens when we have a flush
// call outstanding and we call close.
func (nc *Conn) removeFlushEntry(ch chan bool) bool {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	if nc.pongs == nil {
		return false
	}
	for i, c := range nc.pongs {
		if c == ch {
			nc.pongs[i] = nil
			return true
		}
	}
	return false
}

// The lock must be held entering this function.
func (nc *Conn) sendPing(ch chan bool) {
	nc.pongs = append(nc.pongs, ch)
	nc.bw.WriteString(pingProto)
	nc.kickFlusher()
}

func (nc *Conn) processPingTimer() {
	nc.mu.Lock()

	if nc.status != CONNECTED {
		nc.mu.Unlock()
		return
	}

	// Check for violation
	nc.pout += 1
	if nc.pout > nc.Opts.MaxPingsOut {
		nc.mu.Unlock()
		nc.processOpErr(ErrStaleConnection)
		return
	}

	nc.sendPing(nil)
	nc.ptmr.Reset(nc.Opts.PingInterval)
	nc.mu.Unlock()
}

// FlushTimeout allows a Flush operation to have an associated timeout.
func (nc *Conn) FlushTimeout(timeout time.Duration) (err error) {
	if timeout <= 0 {
		return errors.New("nats: Bad timeout value")
	}

	nc.mu.Lock()
	if nc.isClosed() {
		nc.mu.Unlock()
		return ErrConnectionClosed
	}
	t := time.NewTimer(timeout)
	defer t.Stop()

	ch := make(chan bool) // FIXME: Inefficient?
	nc.sendPing(ch)
	nc.mu.Unlock()

	select {
	case _, ok := <-ch:
		if !ok {
			err = ErrConnectionClosed
		} else {
			nc.mu.Lock()
			err = nc.err
			nc.mu.Unlock()
			close(ch)
		}
	case <-t.C:
		err = ErrTimeout
	}

	if err != nil {
		nc.removeFlushEntry(ch)
	}
	return
}

// Flush will perform a round trip to the server and return when it
// receives the internal reply.
func (nc *Conn) Flush() error {
	return nc.FlushTimeout(60 * time.Second)
}

// resendSubscriptions will send our subscription state back to the
// server. Used in reconnects
func (nc *Conn) resendSubscriptions() {
	for _, s := range nc.subs {
		nc.bw.WriteString(fmt.Sprintf(subProto, s.Subject, s.Queue, s.sid))
		if s.max > 0 {
			maxStr := strconv.Itoa(int(s.max))
			nc.bw.WriteString(fmt.Sprintf(unsubProto, s.sid, maxStr))
		}
	}
}

// Clear pending flush calls and reset
func (nc *Conn) resetPendingFlush() {
	nc.clearPendingFlushCalls()
	nc.pongs = make([]chan bool, 0, 8)
}

// This will clear any pending flush calls and release pending calls.
func (nc *Conn) clearPendingFlushCalls() {
	// Clear any queued pongs, e.g. pending flush calls.
	for _, ch := range nc.pongs {
		if ch != nil {
			ch <- true
		}
	}
	nc.pongs = nil
}

// Low level close call that will do correct cleanup and set
// desired status. Also controls whether user defined callbacks
// will be triggered. The lock should not be held entering this
// function. This function will handle the locking manually.
func (nc *Conn) close(status Status, doCBs bool) {
	nc.mu.Lock()
	if nc.isClosed() {
		nc.status = status
		nc.mu.Unlock()
		return
	}
	nc.status = CLOSED
	nc.mu.Unlock()

	// Kick the Go routines so they fall out.
	// fch will be closed on finalizer
	nc.kickFlusher()

	// Clear any queued pongs, e.g. pending flush calls.
	nc.clearPendingFlushCalls()

	nc.mu.Lock()

	if nc.ptmr != nil {
		nc.ptmr.Stop()
	}

	// Close sync subscriber channels and release any
	// pending NextMsg() calls.
	for _, s := range nc.subs {
		s.mu.Lock()
		if s.mch != nil {
			close(s.mch)
			s.mch = nil
		}
		// Mark as invalid, for signalling to deliverMsgs
		s.mcb = nil
		s.mu.Unlock()
	}
	nc.subs = nil

	// Perform appropriate callback if needed for a disconnect.
	dcb := nc.Opts.DisconnectedCB
	if doCBs && nc.conn != nil && dcb != nil {
		go dcb(nc)
	}

	// Go ahead and make sure we have flushed the outbound buffer.
	nc.status = CLOSED
	if nc.conn != nil {
		nc.bw.Flush()
		nc.conn.Close()
	}
	ccb := nc.Opts.ClosedCB
	nc.mu.Unlock()

	// Perform appropriate callback if needed for a connection closed.
	if doCBs && ccb != nil {
		ccb(nc)
	}
	nc.mu.Lock()
	nc.status = status
	nc.mu.Unlock()
}

// Close will close the connection to the server. This call will release
// all blocking calls, such as Flush() and NextMsg()
func (nc *Conn) Close() {
	nc.close(CLOSED, true)
}

// Test if Conn has been closed.
func (nc *Conn) IsClosed() bool {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	return nc.isClosed()
}

// Test if Conn is reconnecting.
func (nc *Conn) IsReconnecting() bool {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	return nc.isReconnecting()
}

// Status returns the current state of the connection.
func (nc *Conn) Status() Status {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	return nc.status
}

// Test if Conn has been closed Lock is assumed held.
func (nc *Conn) isClosed() bool {
	return nc.status == CLOSED
}

// Test if Conn is being reconnected.
func (nc *Conn) isReconnecting() bool {
	return nc.status == RECONNECTING
}

// Stats will return a race safe copy of the Statistics section for the connection.
func (nc *Conn) Stats() Statistics {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	stats := nc.Statistics
	return stats
}
