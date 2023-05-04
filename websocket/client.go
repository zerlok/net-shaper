package websocket

import (
	"netshaper"
)

type Client = netshaper.Client[*Request, RawResponse]
