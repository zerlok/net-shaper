package http

import (
	"netshaper"
)

type Client = netshaper.Client[*Request, *Response]
