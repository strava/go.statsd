/*The MIT License (MIT)

Copyright (c) 2013 Strava

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
the Software, and to permit persons to whom the Software is furnished to do so,
subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/

// This package implements a small client for StatsD, https://github.com/etsy/statsd
// For detailed documentation and examples see README.md
package statsd

import (
	"bufio"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"
)

var DefaultClient Stater
var DefaultRate float32 = 1.0

var (
	ErrConnectionClosed = errors.New("Connection Closed")
	ErrConnectionWrite  = errors.New("Wrote no bytes")
)

type Stater interface {
	Count(stat string, rate ...float32) error
	Measure(stat string, delta time.Duration, rate ...float32) error
	Gauge(stat string, value interface{}) error
	Close() error
}

// implements Stater
type NoopClient struct{}

// implements Stater
type RemoteClient struct {
	buf    *bufio.ReadWriter // need to read for tests
	conn   *net.Conn
	prefix string
	mutex  sync.Mutex
}

func init() {
	DefaultClient = &NoopClient{}
}

// Creates a new UDP connection to the given server. The prefix
// is optional and will be prepended to any stat using this client.
func New(addr string, prefix ...string) (Stater, error) {
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return nil, err
	}

	p := ""
	if len(prefix) > 0 {
		p = prefix[0]
	}

	buf := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	client := &RemoteClient{buf: buf, conn: &conn, prefix: p}

	return client, nil
}

// Count adds 1 to the provided stat. Rate is optional,
// default is statsd.DefaultRate which is set as 1.0.
// A rate value of 0.1 will only send one in every 10 calls to the
// server. The statsd server will adjust its aggregation accordingly.
func Count(stat string, rate ...float32) error {
	client := DefaultClient
	if client == nil {
		client = &NoopClient{}
	}

	if len(rate) > 0 {
		return client.Count(stat, rate[0])
	}

	return client.Count(stat)
}

// Measure sends a duration to the provided stat (plus the prefix).
// The rate value is optional, default is statsd.DefaultRate which is set as 1.0.
func Measure(stat string, delta time.Duration, rate ...float32) error {
	client := DefaultClient
	if client == nil {
		client = &NoopClient{}
	}

	if len(rate) > 0 {
		return client.Measure(stat, delta, rate[0])
	}

	return client.Measure(stat, delta)
}

// Gauges are arbitrary values that maintain their values' until set to
// something else. Useful for logging queue sizes on set intervals.
func Gauge(stat string, value interface{}) error {
	client := DefaultClient
	if client == nil {
		client = &NoopClient{}
	}

	return client.Gauge(stat, value)
}

// Count adds 1 to the provided stat. Rate is optional,
// default is statsd.DefaultRate which is set as 1.0.
// A rate value of 0.1 will only send one in every 10 calls to the
// server. The statsd server will adjust its aggregation accordingly.
func (self *RemoteClient) Count(stat string, rate ...float32) error {
	r := DefaultRate
	if len(rate) > 0 {
		r = rate[0]
	}

	return self.submit(stat, "1|c", r)
}

// Measure sends a duration to the provided stat (plus the prefix).
// The rate value is optional, default is statsd.DefaultRate which is set as 1.0.
func (s *RemoteClient) Measure(stat string, delta time.Duration, rate ...float32) error {
	r := DefaultRate
	if len(rate) > 0 {
		r = rate[0]
	}

	dap := fmt.Sprintf("%d|ms", int64(delta/time.Millisecond))
	return s.submit(stat, dap, r)
}

// Gauges are arbitrary values that maintain their values' until set to
// something else. Useful for logging queue sizes on set intervals.
func (s *RemoteClient) Gauge(stat string, value interface{}) error {
	dap := fmt.Sprintf("%v|g", value)
	return s.submit(stat, dap, 1)
}

func (self *RemoteClient) Close() error {
	self.buf.Flush()
	self.buf = nil

	return (*self.conn).Close()
}

// submit formats the statsd event data, handles sampling, and prepares it,
// and sends it to the server.
func (self *RemoteClient) submit(stat string, value string, rate float32) error {
	if rate < 1 {
		if rand.Float32() < rate {
			value = fmt.Sprintf("%s|@%f", value, rate)
		} else {
			return nil
		}
	}

	if self.prefix != "" {
		stat = self.prefix + "." + stat
	}

	_, err := self.send([]byte(stat + ":" + value))
	if err != nil {
		return err
	}

	return nil
}

// sends the data to the server endpoint over the net.Conn
func (self *RemoteClient) send(data []byte) (int, error) {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	if self.buf == nil {
		return 0, ErrConnectionClosed
	}

	n, err := self.buf.Write([]byte(data))
	if err != nil {
		return 0, err
	}

	if n == 0 {
		return n, ErrConnectionWrite
	}

	err = self.buf.Flush()
	if err != nil {
		return n, err
	}

	return n, nil
}

func (*NoopClient) Count(stat string, rate ...float32) error {
	return nil
}

func (*NoopClient) Measure(stat string, delta time.Duration, rate ...float32) error {
	return nil
}

func (*NoopClient) Gauge(stat string, value interface{}) error {
	return nil
}

func (*NoopClient) Close() error {
	return nil
}
