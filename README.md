go.statsd
=========

Provides a simple and flexible [StatsD](https://github.com/etsy/statsd) client library
in [Go](http://golang.org). 

#### To install
	
	go get github.com/strava/go.statsd

#### To use, imports as package name `statsd`:

	import "github.com/strava/go.statsd"

<br />
[![Build Status](https://travis-ci.org/strava/go.statsd.png?branch=master)](https://travis-ci.org/strava/go.statsd)
&nbsp; &nbsp;
[![Coverage Status](https://coveralls.io/repos/strava/go.statsd/badge.png?branch=master)](https://coveralls.io/r/strava/go.statsd?branch=master)
&nbsp; &nbsp;
[![Godoc Reference](https://godoc.org/github.com/strava/go.statsd?status.png)](https://godoc.org/github.com/strava/go.statsd)

## Usage

A statsd client defines a UDP connection to the given server. The prefix
is optional and will be prepended to any stat using this client, useful for namespacing
your metrics.

	client, err := statsd.New("statsd-server:8125", "gopher_service")

Suggested usage is to set your client as the package's `statsd.DefaultClient`. 
This allows the convenient usage of package's Count, Measure and Gauge functions
throughout your application.

	statsd.DefaultClient = client

	// usage
	statsd.Count("event")
	statsd.Timer("measurement", time.Millisecond)

The default `statsd.DefaultClient` is a NoopClient that does nothing when called. Specifically, 
it doesn't error since you don't have a connection to the server. 
This allows for easy testing and local development.

### Counters

	func Count(stat string, rate ...float32) error // uses the default client
	func (s *Client) Count(stat string, rate ...float32) error

Count adds 1 to the provided stat (plus the prefix). Rate is optional, a value of 0.1 will send one in
every 10 calls to the server. The statsd server will adjust its counts accordingly.
The default rate is 1.0 and is defined as the package variable `stats.DefaultRate`.

### Timers / Measure

	func Measure(stat string, delta time.Duration, rate ...float32) error
	func (s *Client) Measure(stat string, delta time.Duration, rate ...float32) error

Measure sends a delta time to the provided stat (plus the prefix). The rate value is optional.
Example usage:

	func FunctionToMeasure() {
		start := time.Now()
		defer func() {
			go statsd.Measure("function_time", time.Since(start))
		}()

		// do something you want to measure
	}


### Gauges

	func Gauge(stat string, value interface{}) error
	func (s *Client) Gauge(stat string, value interface{}) error

Gauges are arbitrary values that maintain their values' until set to something else.
Useful for queue sizes.
Example usage:

	// update the queue size every minute in the background
	go func() {
		tick := time.Tick(1 * time.Minute)
		for _ = range tick {
			statsd.Gauge("queue_size", queueSize())
		}
	}()

## Credits

The guys at Etsy for building the [StatsD aggregation daemon](https://github.com/etsy/statsd).
For full description of the client API see [StatsD Metric Types](https://github.com/etsy/statsd/blob/master/docs/metric_types.md).

This code builds on work that came before it: 
[github.com/cactus/go-statsd-client](http://github.com/cactus/go-statsd-client/)
and [github.com/peterbourgon/g2s](http://github.com/peterbourgon/g2s).
