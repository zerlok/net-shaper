package websocket

import (
	"context"
	"errors"
	"netshaper/conf"
	"sync"
)

func NewMockConfig(opts ...conf.Option[*MockConfig]) *MockConfig {
	return conf.ApplyOptionsInit(opts, &MockConfig{
		pendingBuffSize: 1000,
	})
}

func NewMock(ctx context.Context, opts ...conf.Option[*MockConfig]) *Mock {
	return NewMockConfig(opts...).create(ctx)
}

func WithSideEffectsGeneratorFactory(factory func() MockSideEffectGenerator) conf.Option[*MockConfig] {
	return conf.OptionFunc[*MockConfig](func(config *MockConfig) *MockConfig {
		config.genFactory = factory
		return config
	})
}

func WithSideEffectsGenerator(gen MockSideEffectGenerator) conf.Option[*MockConfig] {
	return WithSideEffectsGeneratorFactory(func() MockSideEffectGenerator {
		return gen
	})
}

func WithSideEffects(opts ...conf.Option[*MockResponse]) conf.Option[*MockConfig] {
	return WithSideEffectsGenerator(mockInfiniteSideEffectGenerator(func() []conf.Option[*MockResponse] {
		return opts
	}))
}

func WithSideEffectsForNRequestsFunc(fn func(int) []conf.Option[*MockResponse], n int) conf.Option[*MockConfig] {
	return WithSideEffectsGenerator(&mockFiniteSideEffectGenerator{
		Fn:       fn,
		MaxTimes: n,
	})
}

func WithSideEffectsForNRequests(opts ...[]conf.Option[*MockResponse]) conf.Option[*MockConfig] {
	return WithSideEffectsForNRequestsFunc(func(i int) []conf.Option[*MockResponse] { return opts[i] }, len(opts))
}

func WithSideEffectForSingleRequest(opts ...conf.Option[*MockResponse]) []conf.Option[*MockResponse] {
	return opts
}

func WithRequestError(err error) conf.Option[*MockResponse] {
	return conf.OptionFunc[*MockResponse](func(res *MockResponse) *MockResponse {
		res.requestErr = err
		return res
	})
}
func WithRequestErrorReason(reason string) conf.Option[*MockResponse] {
	return WithRequestError(errors.New(reason))
}

func WithResponseMessages(messages ...Message) conf.Option[*MockResponse] {
	return conf.OptionFunc[*MockResponse](func(res *MockResponse) *MockResponse {
		res.Push(messages...)
		return res
	})
}

func WithResponseClose() conf.Option[*MockResponse] {
	return conf.OptionFunc[*MockResponse](func(res *MockResponse) *MockResponse {
		res.Close(res.ctx)
		return res
	})
}

type MockSideEffectGenerator interface {
	HasNext() bool
	Next() []conf.Option[*MockResponse]
}

type mockInfiniteSideEffectGenerator func() []conf.Option[*MockResponse]

func (g mockInfiniteSideEffectGenerator) HasNext() bool {
	return true
}

func (g mockInfiniteSideEffectGenerator) Next() []conf.Option[*MockResponse] {
	if g == nil {
		return nil
	}

	return g()
}

type mockFiniteSideEffectGenerator struct {
	Fn       func(int) []conf.Option[*MockResponse]
	MaxTimes int
	counter  int
}

func (g *mockFiniteSideEffectGenerator) HasNext() bool {
	return g.counter < g.MaxTimes
}

func (g *mockFiniteSideEffectGenerator) Next() []conf.Option[*MockResponse] {
	if g.Fn == nil {
		return nil
	}

	opts := g.Fn(g.counter)
	g.counter++

	return opts
}

var _ Config = (*MockConfig)(nil)

type MockConfig struct {
	genFactory      func() MockSideEffectGenerator
	pendingBuffSize uint
}

func (c *MockConfig) Create(ctx context.Context) (Client, error) {
	return c.create(ctx), nil
}

func (c *MockConfig) create(ctx context.Context) *Mock {
	var gen MockSideEffectGenerator
	if genFactory := c.genFactory; genFactory != nil {
		gen = genFactory()
	} else {
		gen = mockInfiniteSideEffectGenerator(nil)
	}

	ctx, cancel := context.WithCancel(ctx)
	pending := make(chan *mockJob, c.pendingBuffSize)
	handled := make(chan *MockResponse, cap(pending))

	mock := &Mock{
		ctx:       ctx,
		cancel:    cancel,
		gen:       gen,
		pending:   pending,
		requests:  []*Request{},
		responses: map[*Request]*MockResponse{},
	}

	mock.workersWg.Add(2)
	go mock.handleJobs(pending, handled)
	go mock.saveHandled(handled)

	return mock
}

var _ Client = (*Mock)(nil)

type Mock struct {
	ctx       context.Context
	cancel    context.CancelFunc
	gen       MockSideEffectGenerator
	pending   chan<- *mockJob
	requests  []*Request
	responses map[*Request]*MockResponse
	workersWg sync.WaitGroup
	closeOnce sync.Once
}

func (m *Mock) Create(_ context.Context) (Client, error) {
	return m, nil
}

func (m *Mock) Request(req *Request) (RawResponse, error) {
	job := &mockJob{req, make(chan *MockResponse, 1)}

	select {
	case <-m.ctx.Done():
		close(job.response)
		return nil, m.ctx.Err()
	default:
		m.pending <- job
	}

	res := <-job.response
	if res.requestErr != nil {
		return nil, res.requestErr
	}

	return res, nil
}

func (m *Mock) Close(ctx context.Context) {
	m.closeOnce.Do(func() {
		defer m.workersWg.Wait()
		defer close(m.pending)
		m.cancel()
		for _, res := range m.responses {
			res.Close(ctx)
		}
	})
}

func (m *Mock) Requests() []*Request {
	return m.requests
}

func (m *Mock) Response(req *Request) (res *MockResponse, ok bool) {
	res, ok = m.responses[req]
	return
}

func (m *Mock) handleJobs(pending <-chan *mockJob, handled chan<- *MockResponse) {
	defer m.workersWg.Done()
	defer close(handled)

	for job := range pending {
		sideEffects, ok := m.genSideEffects()
		go m.handleOneJob(job, sideEffects, ok, handled)
	}
}

func (m *Mock) saveHandled(handled <-chan *MockResponse) {
	defer m.workersWg.Done()

	for res := range handled {
		m.requests = append(m.requests, res.request)
		m.responses[res.request] = res
	}
}

func (m *Mock) genSideEffects() (sideEffects []conf.Option[*MockResponse], ok bool) {
	ok = m.gen.HasNext()
	if ok {
		sideEffects = m.gen.Next()
	}

	return
}

func (m *Mock) handleOneJob(job *mockJob, sideEffects []conf.Option[*MockResponse], ok bool, handled chan<- *MockResponse) {
	defer close(job.response)

	response := &MockResponse{
		request: job.request,
	}

	if !ok {
		response.requestErr = errors.New("no side effects left")
	} else {
		resCtx, resCancel := context.WithCancel(job.request.Context())
		response.ctx = resCtx
		response.cancel = resCancel
		response.messages = make(chan Message, job.request.BufferSize)
		response = conf.ApplyOptionsInit(sideEffects, response)
	}

	job.response <- response
	handled <- response
}

var _ RawResponse = (*MockResponse)(nil)

type MockResponse struct {
	request    *Request
	requestErr error
	ctx        context.Context
	cancel     context.CancelFunc
	messages   chan Message
	err        error
	wg         sync.WaitGroup
	closeOnce  sync.Once
}

func (r *MockResponse) Send(message Message) error {
	panic(message)
}

func (r *MockResponse) Listen() <-chan Message {
	return r.messages
}

func (r *MockResponse) Closed() <-chan struct{} {
	return r.ctx.Done()
}

func (r *MockResponse) Err() error {
	return r.err
}

func (r *MockResponse) Close(_ context.Context) {
	r.closeOnce.Do(func() {
		defer close(r.messages)
		r.cancel()
	})
}

func (r *MockResponse) Push(messages ...Message) {
	for _, msg := range messages {
		select {
		case <-r.ctx.Done():
			return
		default:
			r.messages <- msg
		}
	}
}

func (r *MockResponse) PushBytes(messages ...[]byte) {
	for _, msg := range messages {
		select {
		case <-r.ctx.Done():
			return
		default:
			r.messages <- ByteMessage(msg)
		}
	}
}

func (r *MockResponse) PushTexts(messages ...string) {
	for _, msg := range messages {
		select {
		case <-r.ctx.Done():
			return
		default:
			r.messages <- TextMessage(msg)
		}
	}
}

func (r *MockResponse) PushErrors(messages ...error) {
	for _, err := range messages {
		select {
		case <-r.ctx.Done():
			return
		default:
			r.messages <- &ErrorMessage{err}
		}
	}
}

func (r *MockResponse) PushErrorReasons(messages ...string) {
	for _, err := range messages {
		select {
		case <-r.ctx.Done():
			return
		default:
			r.messages <- &ErrorMessage{errors.New(err)}
		}
	}
}

type mockJob struct {
	request  *Request
	response chan *MockResponse
}
