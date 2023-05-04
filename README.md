# NetShaper

NetShaper is a Go library that provides a general interface for making HTTP and WebSocket requests and handling
responses. It's designed to be extensible, allowing other developers to easily add their own custom logic for request
and response processing.

NetShaper includes a flexible client that can be configured with custom transport and proxy settings, as well as
support for request and response middleware. This makes it easy to add common functionality such as authentication, rate
limiting, or logging to your requests.

The library also provides support for streaming requests and responses, as well as WebSocket connections. It includes
utilities for managing connections and handling incoming messages, and can be extended to support custom protocols or
message formats.

NetShaper is fully compatible with Go's standard library and can be easily integrated into existing projects. It's
licensed under the MIT license and contributions are welcome!
