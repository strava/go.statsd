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

// Package statsd implements a small client for StatsD, https://github.com/etsy/statsd
// For detailed documentation and examples see README.md
package statsd

import (
	"bufio"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"
)

// DefaultClient is used by package functions statsd.Count, statsd.Measure, statsd.Gauge functions.
var DefaultClient Stater

// DefaultRate is the rate used for Measure and Count calls if none is provided.
var DefaultRate float32 = 1.0

// DefaultReconnectDelay is the time before trying, yet again, to reconnect after a network error.
var DefaultReconnectDelay = time.Second

var (
	// ErrConnectionClosed is triggered when trying to send on a closed connection.
	// ie. you closed the Client and then tried to send again
	ErrConnectionClosed = errors.New("connection closed")

	// ErrConnectionWrite is returned if there was a problem sending the request.
	ErrConnectionWrite = errors.New("wrote no bytes")
)

// Stater is the interface for posting to StatsD. It is implemented by
// a NoopClient (used for testing and local environments) and the RemoteClient
// which actually creates a connection to the server.
type Stater interface {
	Count(stat string, rate ...float32) error
	CountMultiple(stat string, count int, rate ...float32) error
	Measure(stat string, delta time.Duration, rate ...float32) error
	Gauge(stat string, value interface{}) error
	Close() error
}

// NoopClient implements Stater and is what's used before stats.DefaultClient
// with a real RemoteClient
type NoopClient struct{}

// RemoteClient implements Stater
type RemoteClient struct {
	ReconnectDelay time.Duration
	address        string
	buf            *bufio.ReadWriter // need to read for tests
	conn           net.Conn
	prefix         []byte
	reconnectChan  chan struct{}
	writeMutex     sync.Mutex
}

func init() {
	DefaultClient = NoopClient{}
}

// New opens a new UDP connection to the given server. The prefix
// is optional and will be prepended to any stat using this client.
func New(address string, prefix ...string) (Stater, error) {
	p := ""
	if len(prefix) > 0 {
		p = prefix[0]
	}

	client := &RemoteClient{
		ReconnectDelay: DefaultReconnectDelay,
		address:        address,
		prefix:         []byte(p),
		reconnectChan:  make(chan struct{}, 1),
	}
	client.reconnectChan <- struct{}{}

	err := client.connect()
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (client *RemoteClient) connect() error {
	// here we use client.reconnectChan as a nonblocking mutex.
	select {
	case <-client.reconnectChan:
	default:
		return fmt.Errorf("reconnect delay in progress")
	}

	// release the reconnect lock after the delay
	go func() {
		time.Sleep(client.ReconnectDelay)
		select {
		case client.reconnectChan <- struct{}{}:
		default:
		}
	}()

	client.writeMutex.Lock()
	defer client.writeMutex.Unlock()

	conn, err := net.Dial("udp", client.address)
	if err != nil {
		return err
	}

	client.conn = conn
	client.buf = bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	return nil
}

// Count adds 1 to the provided stat using the statsd.DefaultClient.
func Count(stat string, rate ...float32) error {
	return CountMultiple(stat, 1, rate...)
}

// CountMultiple adds `count` to the provided stat using the statsd.DefaultClient.
func CountMultiple(stat string, count int, rate ...float32) error {
	client := DefaultClient
	if client == nil {
		client = NoopClient{}
	}

	return client.CountMultiple(stat, count, rate...)

}

// Measure reports a duration using the statsd.DefaultClient client.
func Measure(stat string, delta time.Duration, rate ...float32) error {
	client := DefaultClient
	if client == nil {
		client = NoopClient{}
	}

	return client.Measure(stat, delta, rate...)
}

// Gauge set a StatsD gauge value using the statsd.DefaultClient client.
func Gauge(stat string, value interface{}) error {
	client := DefaultClient
	if client == nil {
		client = NoopClient{}
	}

	return client.Gauge(stat, value)
}

// Count adds 1 to the provided stat. Rate is optional,
// default is statsd.DefaultRate which is set as 1.0.
// A rate value of 0.1 will only send one in every 10 calls to the
// server. The statsd server will adjust its aggregation accordingly.
func (client *RemoteClient) Count(stat string, rate ...float32) error {
	return client.CountMultiple(stat, 1, rate...)
}

// CountMultiple adds `count` to the provided stat. Rate is optional,
// default is statsd.DefaultRate which is set as 1.0.
// A rate value of 0.1 will only send one in every 10 calls to the
// server. The statsd server will adjust its aggregation accordingly.
func (client *RemoteClient) CountMultiple(stat string, count int, rate ...float32) error {
	r := DefaultRate
	if len(rate) > 0 {
		r = rate[0]
	}

	// fmt.Sprintf("%d|c", count)
	data := make([]byte, 0, 20)
	data = strconv.AppendInt(data, int64(count), 10)
	data = append(data, '|', 'c')

	return client.submit(stat, data, r)
}

// Measure reports a duration to the provided stat (plus the prefix).
// The rate value is optional, default is statsd.DefaultRate which is set as 1.0.
func (client *RemoteClient) Measure(stat string, delta time.Duration, rate ...float32) error {
	r := DefaultRate
	if len(rate) > 0 {
		r = rate[0]
	}

	// data := fmt.Sprintf("%d|ms", int64(delta/time.Millisecond))
	data := make([]byte, 0, 20)
	data = strconv.AppendInt(data, int64(delta/time.Millisecond), 10)
	data = append(data, '|', 'm', 's')

	return client.submit(stat, data, r)
}

// Gauge set a StatsD gauge value which is an arbitrary value that maintain
// its value until set to something else.
// Useful for logging queue sizes on set intervals.
func (client *RemoteClient) Gauge(stat string, value interface{}) error {
	dap := fmt.Sprintf("%v|g", value)
	return client.submit(stat, []byte(dap), 1)
}

// Close flushes the buffer and closes the connection.
func (client *RemoteClient) Close() error {
	client.buf.Flush()
	client.buf = nil

	return client.conn.Close()
}

// submit formats the statsd event data, handles sampling, and prepares it,
// and sends it to the server.
func (client *RemoteClient) submit(stat string, value []byte, rate float32) error {
	if rate < 1 {
		if rand.Float32() < rate {
			// value = fmt.Sprintf("%s|@%f", value, rate)
			value = append(value, '|', '@')
			value = strconv.AppendFloat(value, float64(rate), 'f', -1, 32)
		} else {
			return nil
		}
	}

	message := make([]byte, 0, len(client.prefix)+len(stat)+len(value)+3)

	if len(client.prefix) != 0 {
		message = append(message, client.prefix...)
		message = append(message, '.')
	}

	// This loop removes the need for the intermediate string -> []byte convertion into append
	// message = append(message, []byte(stat)...)
	for i := 0; i < len(stat); i++ {
		message = append(message, stat[i])
	}

	message = append(message, ':')
	message = append(message, value...)

	_, err := client.send(message)
	if err != nil {
		connectError := client.connect()

		// reconnect successed so try again with this one.
		if connectError == nil {
			_, err := client.send(message)
			return err
		}

		// reconnect delay in progress so send the original error.
		return err
	}

	return nil
}

// sends the data to the server endpoint over the net.Conn
func (client *RemoteClient) send(data []byte) (int, error) {
	client.writeMutex.Lock()
	defer client.writeMutex.Unlock()

	if client.buf == nil {
		return 0, ErrConnectionClosed
	}

	n, err := client.buf.Write(data)
	if err != nil {
		return 0, err
	}

	if n == 0 {
		return n, ErrConnectionWrite
	}

	// TOOD: figure out if we really need to do a buffer flush after every metric.
	err = client.buf.Flush()
	if err != nil {
		return n, err
	}

	return n, nil
}

// Count on NoopClient is a noop and does not require and internet connection.
func (NoopClient) Count(stat string, rate ...float32) error {
	return nil
}

// CountMultiple on NoopClient is a noop and does not require and internet connection.
func (NoopClient) CountMultiple(stat string, count int, rate ...float32) error {
	return nil
}

// Measure on NoopClient is a noop and does not require and internet connection.
func (NoopClient) Measure(stat string, delta time.Duration, rate ...float32) error {
	return nil
}

// Gauge on NoopClient is a noop and does not require and internet connection.
func (NoopClient) Gauge(stat string, value interface{}) error {
	return nil
}

// Close on NoopClient is a noop and does not require and internet connection.
func (NoopClient) Close() error {
	return nil
}
