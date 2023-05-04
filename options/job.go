package options

import (
	net_shaper "netshaper"
)

type requestJob[T1 net_shaper.Request, T2 any] struct {
	request T1
	results chan<- *responseResult[T2]
}

type responseResult[T any] struct {
	response T
	err      error
}
