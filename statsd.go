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

var DefaultClient *Client
var DefaultRate float32 = 1.0

var (
	ConnectionClosedErr = errors.New("Connection Closed")
	ConnectionWriteErr  = errors.New("Wrote no bytes")
)

type Client struct {
	buf    *bufio.ReadWriter // need to read for tests
	conn   *net.Conn
	prefix string
	mutex  sync.Mutex
}

// Creates a new UDP connection to the given server. The prefix
// is optional and will be prepended to any stat using this client.
func New(addr string, prefix ...string) (*Client, error) {
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return nil, err
	}

	p := ""
	if len(prefix) > 0 {
		p = prefix[0]
	}

	buf := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	client := &Client{buf: buf, conn: &conn, prefix: p}

	return client, nil
}

// Count adds 1 to the provided stat. Rate is optional,
// default is statsd.DefaultRate which is set as 1.0.
// A rate value of 0.1 will only send one in every 10 calls to the
// server. The statsd server will adjust its aggregation accordingly.
func Count(stat string, rate ...float32) error {
	if len(rate) > 0 {
		return DefaultClient.Count(stat, rate[0])
	}

	return DefaultClient.Count(stat)
}

// Measure sends a duration to the provided stat (plus the prefix).
// The rate value is optional, default is statsd.DefaultRate which is set as 1.0.
func Measure(stat string, delta time.Duration, rate ...float32) error {
	if len(rate) > 0 {
		return DefaultClient.Measure(stat, delta, rate[0])
	}

	return DefaultClient.Measure(stat, delta)
}

// Gauges are arbitrary values that maintain their values' until set to
// something else. Useful for logging queue sizes on set intervals.
func Gauge(stat string, value interface{}) error {
	return DefaultClient.Gauge(stat, value)
}

func (s *Client) Close() error {
	s.buf.Flush()
	s.buf = nil

	return (*s.conn).Close()
}

// Count adds 1 to the provided stat. Rate is optional,
// default is statsd.DefaultRate which is set as 1.0.
// A rate value of 0.1 will only send one in every 10 calls to the
// server. The statsd server will adjust its aggregation accordingly.
func (s *Client) Count(stat string, rate ...float32) error {
	r := DefaultRate
	if len(rate) > 0 {
		r = rate[0]
	}

	return s.submit(stat, "1|c", r)
}

// Measure sends a duration to the provided stat (plus the prefix).
// The rate value is optional, default is statsd.DefaultRate which is set as 1.0.
func (s *Client) Measure(stat string, delta time.Duration, rate ...float32) error {
	r := DefaultRate
	if len(rate) > 0 {
		r = rate[0]
	}

	dap := fmt.Sprintf("%d|ms", int64(delta/time.Millisecond))
	return s.submit(stat, dap, r)
}

// Gauges are arbitrary values that maintain their values' until set to
// something else. Useful for logging queue sizes on set intervals.
func (s *Client) Gauge(stat string, value interface{}) error {
	dap := fmt.Sprintf("%v|g", value)
	return s.submit(stat, dap, 1)
}

// submit formats the statsd event data, handles sampling, and prepares it,
// and sends it to the server.
func (s *Client) submit(stat string, value string, rate float32) error {
	if rate < 1 {
		if rand.Float32() < rate {
			value = fmt.Sprintf("%s|@%f", value, rate)
		} else {
			return nil
		}
	}

	if s.prefix != "" {
		stat = s.prefix + "." + stat
	}

	_, err := s.send([]byte(stat + ":" + value))
	if err != nil {
		return err
	}

	return nil
}

// sends the data to the server endpoint over the net.Conn
func (s *Client) send(data []byte) (int, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.buf == nil {
		return 0, ConnectionClosedErr
	}

	n, err := s.buf.Write([]byte(data))
	if err != nil {
		return 0, err
	}

	if n == 0 {
		return n, ConnectionWriteErr
	}

	err = s.buf.Flush()
	if err != nil {
		return n, err
	}

	return n, nil
}
