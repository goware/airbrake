package airbrake

import "time"

type Config struct {
	MaxQueue int
	MaxConn  int
	Timeout  time.Duration

	Transport Transport

	/*
		RetryAfter time.Duration
		TTL        time.Duration

		RetryHandler func(*Event)
		DropHandler  func(*Event)*/
}

var configDefaults = Config{
	MaxQueue: 100,
	MaxConn:  1,
	Timeout:  30 * time.Second,
}

func (c *Config) SetDefaults() {
	emptyConf := Config{}

	if c.MaxQueue == emptyConf.MaxQueue {
		c.MaxQueue = configDefaults.MaxQueue
	}
	if c.MaxQueue < 1 {
		c.MaxQueue = 1
	}

	if c.MaxConn == emptyConf.MaxConn {
		c.MaxConn = configDefaults.MaxConn
	}
	if c.MaxConn < 1 {
		c.MaxConn = 1
	}

	if c.Timeout == emptyConf.Timeout {
		c.Timeout = configDefaults.Timeout
	}
	if c.Timeout < time.Millisecond {
		c.Timeout = time.Microsecond
	}
}
