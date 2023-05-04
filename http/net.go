package http

import (
	"context"
	"fmt"
	"net/http"
	"netshaper/conf"
	"time"
)

func NewNet(opts ...conf.Option[NetConfig]) conf.Option[Config] {
	return conf.OptionFunc[Config](func(config Config) Config {
		if config != nil {
			panic(fmt.Errorf("net option received non-nil config %#v", config))
		}

		cfg := conf.ApplyOptions(opts)
		return &cfg
	})
}

func WithNetClient(client *http.Client) conf.Option[NetConfig] {
	return conf.OptionFunc[NetConfig](func(config NetConfig) NetConfig {
		config.Client = *client
		return config
	})
}

func WithNetTransport(transport http.RoundTripper) conf.Option[NetConfig] {
	return conf.OptionFunc[NetConfig](func(config NetConfig) NetConfig {
		config.Client.Transport = transport
		return config
	})
}

func WithNetCheckRedirect(checkRedirect func(req *http.Request, via []*http.Request) error) conf.Option[NetConfig] {
	return conf.OptionFunc[NetConfig](func(config NetConfig) NetConfig {
		config.Client.CheckRedirect = checkRedirect
		return config
	})
}

func WithNetJar(jar http.CookieJar) conf.Option[NetConfig] {
	return conf.OptionFunc[NetConfig](func(config NetConfig) NetConfig {
		config.Client.Jar = jar
		return config
	})
}

func WithNetTimeout(timeout time.Duration) conf.Option[NetConfig] {
	return conf.OptionFunc[NetConfig](func(config NetConfig) NetConfig {
		config.Client.Timeout = timeout
		return config
	})
}

var _ Config = (*NetConfig)(nil)

type NetConfig struct {
	Client http.Client
}

var _ Client = (*netClient)(nil)

type netClient struct {
	client *http.Client
}

func (c *NetConfig) Create(_ context.Context) (Client, error) {
	return &netClient{
		client: &http.Client{
			Transport:     c.Client.Transport,
			CheckRedirect: c.Client.CheckRedirect,
			Jar:           c.Client.Jar,
			Timeout:       c.Client.Timeout,
		},
	}, nil
}

func (c *netClient) Request(req *Request) (res *Response, err error) {
	res, err = c.client.Do(req)
	return
}

func (c *netClient) Close(_ context.Context) {
}
