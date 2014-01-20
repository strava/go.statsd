// https://github.com/etsy/statsd/blob/master/docs/metric_types.md
package statsd

import (
	"bufio"
	"bytes"
	"testing"
	"time"
)

func NewTestClient(prefix string) (*Client, *bytes.Buffer) {
	b := &bytes.Buffer{}
	buf := bufio.NewReadWriter(bufio.NewReader(b), bufio.NewWriter(b))
	c := &Client{buf: buf, prefix: prefix}
	return c, b
}

func TestNew(t *testing.T) {
	// invalid address
	c, err := New("0.0.0.0")
	if err == nil {
		t.Error("invalid address, should have returned error")
	}

	// without prefix
	c, err = New("0.0.0.0:1000")
	if err != nil {
		t.Fatal(err)
	}

	b := &bytes.Buffer{}
	c.buf = bufio.NewReadWriter(bufio.NewReader(b), bufio.NewWriter(b))

	c.Count("test")
	expected := "test:1|c"
	if b := b.String(); b != expected {
		t.Fatalf("expected %s, got %s", expected, b)
	}

	// with prefix
	c2, err := New("0.0.0.0:1000", "prefix")
	if err != nil {
		t.Fatal(err)
	}

	b.Reset()
	c2.buf = c.buf

	c2.Count("test")
	expected = "prefix.test:1|c"
	if b := b.String(); b != expected {
		t.Fatalf("expected %s, got %s", expected, b)
	}
}

func TestClose(t *testing.T) {
	c, err := New("0.0.0.0:1000")
	if err != nil {
		t.Fatal(err)
	}

	c.Close()

	err = c.Count("test")
	if err != ConnectionClosedErr {
		t.Error("closed connection, should have returned ConnectionClosedErr")
	}

}

func TestCount(t *testing.T) {
	c, buf := NewTestClient("default")
	DefaultClient = c

	err := Count("count")
	if err != nil {
		t.Fatal(err)
	}

	expected := "default.count:1|c"
	if b := buf.String(); b != expected {
		t.Fatalf("expected %s, got %s", expected, b)
	}

	// with rate
	buf.Reset()
	err = Count("count", 0.999999)
	if err != nil {
		t.Fatal(err)
	}

	expected = "default.count:1|c|@0.999999"
	if b := buf.String(); b != expected {
		t.Fatalf("expected %s, got %s", expected, b)
	}
}

func TestClientCount(t *testing.T) {
	c, buf := NewTestClient("test")

	err := c.Count("count")
	if err != nil {
		t.Fatal(err)
	}

	expected := "test.count:1|c"
	if b := buf.String(); b != expected {
		t.Fatalf("expected %s, got %s", expected, b)
	}

	// with rate
	buf.Reset()
	err = c.Count("count", 0.999999)
	if err != nil {
		t.Fatal(err)
	}

	expected = "test.count:1|c|@0.999999"
	if b := buf.String(); b != expected {
		t.Fatalf("expected %s, got %s", expected, b)
	}

	// below rate, should not fill buffer
	buf.Reset()
	err = c.Count("count", 0.0)
	if err != nil {
		t.Fatal(err)
	}

	expected = ""
	if b := buf.String(); b != expected {
		t.Fatalf("expected %s, got %s", expected, b)
	}
}

func TestClientMeasure(t *testing.T) {
	c, buf := NewTestClient("test")

	// test standard
	err := c.Measure("timing", time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	expected := "test.timing:1|ms"
	if b := buf.String(); b != expected {
		t.Fatalf("expected %s, got %s", expected, b)
	}

	// test not even millisecond
	buf.Reset()
	err = c.Measure("timing", 2800*time.Microsecond)
	if err != nil {
		t.Fatal(err)
	}

	expected = "test.timing:2|ms"
	if b := buf.String(); b != expected {
		t.Fatalf("expected %s, got %s", expected, b)
	}

	// with rate
	buf.Reset()
	err = c.Measure("measure", time.Second, 0.999999)
	if err != nil {
		t.Fatal(err)
	}

	expected = "test.measure:1000|ms|@0.999999"
	if b := buf.String(); b != expected {
		t.Fatalf("expected %s, got %s", expected, b)
	}
}

func TestMeasure(t *testing.T) {
	c, buf := NewTestClient("default")
	DefaultClient = c

	// test standard
	err := Measure("timing", time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	expected := "default.timing:1|ms"
	if b := buf.String(); b != expected {
		t.Fatalf("expected %s, got %s", expected, b)
	}

	// with rate
	buf.Reset()
	err = Measure("measure", time.Second, 0.999999)
	if err != nil {
		t.Fatal(err)
	}

	expected = "default.measure:1000|ms|@0.999999"
	if b := buf.String(); b != expected {
		t.Fatalf("expected %s, got %s", expected, b)
	}
}

func TestGauge(t *testing.T) {
	c, buf := NewTestClient("stub")
	DefaultClient = c

	err := Gauge("measure", 10.5)
	if err != nil {
		t.Fatal(err)
	}

	expected := "stub.measure:10.5|g"
	if b := buf.String(); b != expected {
		t.Fatalf("expected %s, got %s", expected, b)
	}
}

func TestClientGauge(t *testing.T) {
	c, buf := NewTestClient("stub")

	err := c.Gauge("measure", 10)
	if err != nil {
		t.Fatal(err)
	}

	expected := "stub.measure:10|g"
	if b := buf.String(); b != expected {
		t.Fatalf("expected %s, got %s", expected, b)
	}
}

func TestEmptyPrefix(t *testing.T) {
	c, buf := NewTestClient("")

	err := c.Count("c", 1.0)
	if err != nil {
		t.Fatal(err)
	}

	expected := "c:1|c"
	if b := buf.String(); b != expected {
		t.Fatalf("expected %s, got %s", expected, b)
	}
}
