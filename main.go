package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"fyne.io/systray"
	"github.com/grandcat/zeroconf"
)

const (
	serviceType     = "_cliplink._tcp"
	domain          = "local."
	pollInterval    = 300 * time.Millisecond
	reconnectDelay  = 5 * time.Second
	maxFrameSize    = 64 << 20 // 64 MB
	recentCacheSize = 128
)

// Node manages all peers and clipboard state.
type Node struct {
	id         string
	port       int
	mu         sync.Mutex
	conns      map[string]net.Conn
	lastHash   [32]byte
	recentMsgs []string // rolling window of seen message IDs
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

func newNode(port int) *Node {
	b := make([]byte, 8)
	rand.Read(b)
	return &Node{
		id:    hex.EncodeToString(b),
		port:  port,
		conns: make(map[string]net.Conn),
	}
}

// seenRecently returns true if msgID was already seen; otherwise records it.
// Handles both echo prevention and duplicate-connection dedup in one mechanism.
func (n *Node) seenRecently(msgID string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, v := range n.recentMsgs {
		if v == msgID {
			return true
		}
	}
	if len(n.recentMsgs) >= recentCacheSize {
		n.recentMsgs = n.recentMsgs[1:]
	}
	n.recentMsgs = append(n.recentMsgs, msgID)
	return false
}

// Frame layout: [4B total-payload-len (BE)] [16B msgID] [data...]
func writeFrame(conn net.Conn, msgID [16]byte, data []byte) error {
	buf := make([]byte, 4+16+len(data))
	binary.BigEndian.PutUint32(buf[:4], uint32(16+len(data)))
	copy(buf[4:20], msgID[:])
	copy(buf[20:], data)
	_, err := conn.Write(buf)
	return err
}

func readFrame(conn net.Conn) ([16]byte, []byte, error) {
	var msgID [16]byte
	hdr := make([]byte, 4)
	if _, err := io.ReadFull(conn, hdr); err != nil {
		return msgID, nil, err
	}
	size := binary.BigEndian.Uint32(hdr)
	if size < 16 || size > maxFrameSize {
		return msgID, nil, fmt.Errorf("bad frame size: %d", size)
	}
	frame := make([]byte, size)
	if _, err := io.ReadFull(conn, frame); err != nil {
		return msgID, nil, err
	}
	copy(msgID[:], frame[:16])
	return msgID, frame[16:], nil
}

func newMsgID() [16]byte {
	var id [16]byte
	rand.Read(id[:])
	return id
}

func hexID(id [16]byte) string {
	return hex.EncodeToString(id[:])
}

// broadcast sends data to all connected peers, pruning dead connections.
func (n *Node) broadcast(data []byte, msgID [16]byte) {
	n.mu.Lock()
	defer n.mu.Unlock()
	var dead []string
	for key, conn := range n.conns {
		if err := writeFrame(conn, msgID, data); err != nil {
			log.Printf("send to %s failed: %v", key, err)
			conn.Close()
			dead = append(dead, key)
		}
	}
	for _, k := range dead {
		delete(n.conns, k)
	}
}

func (n *Node) addConn(key string, conn net.Conn) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	if _, ok := n.conns[key]; ok {
		return false
	}
	n.conns[key] = conn
	return true
}

func (n *Node) removeConn(key string) {
	n.mu.Lock()
	delete(n.conns, key)
	n.mu.Unlock()
}

func (n *Node) connCount() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.conns)
}

func (n *Node) handleConn(key string, conn net.Conn) {
	defer func() {
		conn.Close()
		n.removeConn(key)
		log.Printf("peer disconnected: %s", key)
	}()
	log.Printf("peer connected: %s", key)
	for {
		msgID, data, err := readFrame(conn)
		if err != nil {
			if err != io.EOF {
				log.Printf("read %s: %v", key, err)
			}
			return
		}
		if n.seenRecently(hexID(msgID)) {
			continue
		}

		s := string(data)
		log.Printf("recv from %s (%d bytes) preview=%q", key, len(data), truncate(s, 40))
		if err := writeClipboard(s); err != nil {
			log.Printf("clipboard write: %v", err)
			continue
		}
		// Read back what the OS actually stored so our hash matches what
		// pollClipboard will see (line-endings may differ across platforms).
		actual, readErr := readClipboard()
		var h [32]byte
		if readErr == nil && actual != "" {
			h = sha256.Sum256([]byte(actual))
		} else {
			h = sha256.Sum256(data)
		}
		n.mu.Lock()
		n.lastHash = h
		n.mu.Unlock()
		log.Printf("clipboard updated from %s", key)
	}
}

func (n *Node) pollClipboard() {
	// Seed lastHash with current content so we don't broadcast stale clipboard on startup.
	if text, err := readClipboard(); err == nil && text != "" {
		n.mu.Lock()
		n.lastHash = sha256.Sum256([]byte(text))
		n.mu.Unlock()
	}

	for {
		time.Sleep(pollInterval)
		text, err := readClipboard()
		if err != nil || text == "" {
			continue
		}
		h := sha256.Sum256([]byte(text))
		n.mu.Lock()
		same := n.lastHash == h
		if !same {
			n.lastHash = h
		}
		n.mu.Unlock()
		if same {
			continue
		}
		msgID := newMsgID()
		n.seenRecently(hexID(msgID)) // prevent our own message echoing back
		n.broadcast([]byte(text), msgID)
		log.Printf("broadcasted %d bytes preview=%q", len(text), truncate(text, 40))
	}
}

// connectToPeer dials addr and retries forever on failure or disconnect.
func (n *Node) connectToPeer(addr string) {
	for {
		n.mu.Lock()
		_, exists := n.conns[addr]
		n.mu.Unlock()
		if exists {
			time.Sleep(reconnectDelay)
			continue
		}
		conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
		if err != nil {
			log.Printf("dial %s: %v", addr, err)
			time.Sleep(reconnectDelay)
			continue
		}
		if !n.addConn(addr, conn) {
			conn.Close()
			time.Sleep(reconnectDelay)
			continue
		}
		n.handleConn(addr, conn)
		time.Sleep(reconnectDelay)
	}
}

// browsePeers continuously discovers peers via mDNS and spawns connectors.
func (n *Node) browsePeers() {
	var known sync.Map
	for {
		resolver, err := zeroconf.NewResolver(nil)
		if err != nil {
			log.Printf("mDNS resolver: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		entries := make(chan *zeroconf.ServiceEntry)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

		go func() {
			for entry := range entries {
				for _, txt := range entry.Text {
					if txt == "id="+n.id {
						goto nextEntry // skip self
					}
				}
				for _, ip := range entry.AddrIPv4 {
					addr := fmt.Sprintf("%s:%d", ip, entry.Port)
					if _, loaded := known.LoadOrStore(addr, true); !loaded {
						log.Printf("discovered peer: %s", addr)
						go n.connectToPeer(addr)
					}
				}
			nextEntry:
			}
		}()

		if err := resolver.Browse(ctx, serviceType, domain, entries); err != nil {
			log.Printf("mDNS browse: %v", err)
		}
		<-ctx.Done()
		cancel()
	}
}

func (n *Node) acceptConnections(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		key := conn.RemoteAddr().String()
		if !n.addConn(key, conn) {
			conn.Close()
			continue
		}
		go n.handleConn(key, conn)

		// Spawn a reverse connectToPeer so we have a persistent reconnect loop
		// even if mDNS never discovers this peer.
		remoteIP, _, splitErr := net.SplitHostPort(key)
		if splitErr == nil {
			reverseAddr := net.JoinHostPort(remoteIP, fmt.Sprintf("%d", n.port))
			go n.connectToPeer(reverseAddr)
		}
	}
}

var appPort int

func main() {
	portFlag := flag.Int("port", 56789, "TCP port to listen on")
	flag.Parse()
	log.SetFlags(log.Ltime | log.Lshortfile)
	appPort = *portFlag

	systray.Run(onReady, onExit)
}

func onReady() {
	hideDockIcon()

	systray.SetIcon(iconData)
	systray.SetTooltip("ClipLink")

	mStatus := systray.AddMenuItem("ClipLink 运行中", "")
	mStatus.Disable()
	systray.AddSeparator()

	mAutostart := systray.AddMenuItem("", "开机时自动启动")
	updateAutostartItem(mAutostart)

	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出", "退出 ClipLink")

	go func() {
		for {
			select {
			case <-mAutostart.ClickedCh:
				var err error
				if isAutostartEnabled() {
					err = disableAutostart()
				} else {
					err = enableAutostart()
				}
				if err != nil {
					log.Println(autostartErrorFmt(err))
				}
				updateAutostartItem(mAutostart)
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()

	go startClipLink()
}

func updateAutostartItem(m *systray.MenuItem) {
	if isAutostartEnabled() {
		m.SetTitle("✓ 开机自启")
	} else {
		m.SetTitle("  开机自启")
	}
}

func onExit() {
	os.Exit(0)
}

func startClipLink() {
	node := newNode(appPort)

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", appPort))
	if err != nil {
		log.Fatalf("listen :%d: %v", appPort, err)
	}
	log.Printf("cliplink started | id=%s port=%d", node.id, appPort)

	hostname, _ := os.Hostname()
	server, err := zeroconf.Register(
		fmt.Sprintf("%s-%s", hostname, node.id),
		serviceType, domain, appPort,
		[]string{"id=" + node.id, "v=1"},
		nil,
	)
	if err != nil {
		log.Fatalf("mDNS register: %v", err)
	}
	defer server.Shutdown()

	go node.acceptConnections(ln)
	go node.browsePeers()
	node.pollClipboard()
}
