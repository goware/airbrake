package airbrake

import (
	"log"
	"sync"
	"time"
)

type Transport interface {
	Send(*Event, time.Duration) error
}

type Client struct {
	transport Transport
	timeout   time.Duration
	wg        sync.WaitGroup

	queue chan *Event
	/*
		retryHandler func(*Event)
		dropHandler  func(*Event)*/
}

func New(conf *Config) *Client {
	if conf == nil {
		conf = &configDefaults
	}
	conf.SetDefaults()

	c := Client{
		queue:     make(chan *Event, conf.MaxQueue),
		timeout:   conf.Timeout,
		transport: conf.Transport,
	}

	// setup transport here

	for i := 0; i < conf.MaxConn; i++ {
		c.wg.Add(1)
		go c.worker()
	}

	return &c
}

func (c *Client) Close() {
	close(c.queue)
	c.wg.Wait()
}

func (c *Client) send(e *Event) {
	// Overflowing Queue - if channel buffer is full drop event
	select {
	case c.queue <- e:
	default:
		LogDropHandler(e)
	}
}

func (c *Client) blockingSend(e *Event) error {
	return c.transport.Send(e, c.timeout)
}

func (c *Client) worker() {
	defer c.wg.Done()
	for e := range c.queue {
		if err := c.transport.Send(e, c.timeout); err != nil {
			// if sending fails return to queue for retry (needs max retries)
			// c.send(e)
		}
	}
}

func LogDropHandler(e *Event) {
	log.Println("Max queue backlog exceeded. Dropping event.")
}

/*
func NoRetryHandler(e *Event) {}
*/
