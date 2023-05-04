package http

import "netshaper"

type Config = netshaper.Config[*Request, *Response]
