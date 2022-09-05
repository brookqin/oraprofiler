package main

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"
	"github.com/lithammer/shortuuid/v4"
	"golang.org/x/text/encoding"
	"golang.org/x/text/transform"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512

	sqlRegex = `PARSING\sIN\sCURSOR[\w\W]*tim=(?P<tim>\d+)[\w\W]*sqlid='(?P<id>\w+)'`
)

var (
	newline      = []byte{'\n'}
	status       = &TraceStatus{sid: 0, serial: 0, service: "", tracing: false, lastOffset: 0}
	stopWatching = make(chan bool)
	re           = regexp.MustCompile(sqlRegex)
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	server *Server

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte
	Id   string
	sync.Mutex
}

type Command struct {
	Command string      `json:"command"`
	Data    interface{} `json:"data"`
}

type TraceStatus struct {
	sid        int
	serial     int
	service    string
	traceFile  string
	tracing    bool
	lastOffset int64
	encoding   encoding.Encoding
}

type Trace struct {
	Id  string `json:"id"`
	Sql string `json:"sql"`
	Tim int64  `json:"tim"`
}

func (c *Client) readPump() {
	defer func() {
		c.server.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("ping error: %v", err)
			}
			break
		}

		c.processCommand(string(message))
	}
}

func (s *Client) processCommand(cmd string) {
	switch {
	case strings.HasPrefix(cmd, "trace"):
		args := strings.Split(cmd, "|")
		if len(args) != 4 {
			break
		}

		sid, err := strconv.Atoi(args[1])
		if err != nil {
			s.sendJson(&Command{Command: "trace", Data: "stopped"})
			return
		}

		serial, err := strconv.Atoi(args[2])
		if err != nil {
			s.sendJson(&Command{Command: "trace", Data: "stopped"})
			return
		}
		service := args[3]

		if status.sid == sid && status.serial == serial && status.tracing {
			s.sendJson(&Command{Command: "trace", Data: "started"})
			return
		}

		s.startTracing(sid, serial, service)
	case cmd == "untrace":
		if !status.tracing {
			s.sendJson(&Command{"trace", "stopped"})
			return
		}
		s.stopTracing()
	}
}

func (c *Client) startTracing(sid int, serial int, service string) {
	if status.tracing {
		c.server.DisableTrace(status.sid, status.serial)
		status.tracing = false
	}

	status = &TraceStatus{sid: sid, serial: serial, service: service, tracing: false}
	status.traceFile = c.server.GetTraceFile(sid)
	status.encoding = c.server.GetEncoding(service)
	if status.traceFile != "" {
		c.server.EnableTrace(sid, serial)
		log.Println("start tracing", status.traceFile)
		fileChanged(status.traceFile, c)
		c.sendJson(&Command{Command: "trace", Data: "started"})
	} else {
		c.sendJson(&Command{Command: "trace", Data: "stopped"})
	}
}

func matchGroups(txt string) map[string]string {
	match := re.FindStringSubmatch(txt)
	if match == nil {
		return nil
	}

	groups := make(map[string]string)
	for i, name := range re.SubexpNames() {
		if i != 0 && name != "" {
			groups[name] = match[i]
		}
	}

	return groups
}

func fileChanged(path string, client *Client) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Panicln("init watcher: ", err)
		return
	}

	go func() {
		defer func() {
			watcher.Close()
			log.Println("close watcher")
		}()
		for {
			select {
			// watch for events
			case event := <-watcher.Events:
				if event.Op&fsnotify.Write == fsnotify.Write && event.Name == path {
					client.Lock()

					f, err := os.Open(path)
					if err != nil {
						return
					}
					// TODO: 解决编码问题
					// select VALUE from nls_database_parameters where parameter='NLS_CHARACTERSET'
					f.Seek(int64(status.lastOffset), io.SeekStart)

					var reader io.Reader
					if status.encoding == nil {
						reader = f
					} else {
						reader = transform.NewReader(f, status.encoding.NewDecoder())
					}

					sc := bufio.NewScanner(reader)

					willReadSql := false
					var t *Trace
					for sc.Scan() {
						line := sc.Text()
						if willReadSql {
							if strings.HasPrefix(line, "END OF STMT") {
								willReadSql = false
								client.sendJson(&Command{Command: "trace", Data: *t})
								t = nil
							} else {
								t.Sql += line + "\n"
							}
						} else {
							if groups := matchGroups(line); groups != nil {
								willReadSql = true
								tim, err := strconv.ParseInt(groups["tim"], 10, 64)
								if err != nil {
									tim = 0
								}
								t = &Trace{Id: groups["id"], Sql: "", Tim: tim}
							}
						}
					}

					offset, err := f.Seek(0, io.SeekCurrent)
					if err == nil {
						status.lastOffset = offset
					}

					f.Close()

					client.Unlock()
				}
				// watch for errors
			case err := <-watcher.Errors:
				if err != nil {
					log.Panicln("watcher error: ", err)
				}
			case <-stopWatching:
				return
			}
		}
	}()

	if err := watcher.Add(filepath.Dir(path)); err != nil {
		log.Panicln("watcher add:", err, path)
	}
}

func (c *Client) stopTracing() {
	stopWatching <- true
	status.tracing = false
	c.server.DisableTrace(status.sid, status.serial)
	c.sendJson(&Command{Command: "trace", Data: "stopped"})
}

func (s *Client) sendJson(cmd *Command) {
	if b, err := json.Marshal(cmd); err == nil {
		s.send <- b
	} else {
		log.Printf("send json error: %v", err)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func serveWs(server *Server, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("serveWs", err)
		return
	}
	client := &Client{server: server, conn: conn, send: make(chan []byte, 256), Id: shortuuid.New()}
	client.server.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()
}
