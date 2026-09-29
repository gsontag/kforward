package editor

import (
	"context"
	"strconv"
	"time"

	"gsontag.fr/kforward/internal/kube"
)

// Source answers the questions of the Suggester, by querying the cluster of
// the given kubeconfig context; an empty namespace is the default one of the
// context.
type Source interface {
	Namespaces(ctx context.Context, kubeContext string) ([]string, error)
	Targets(ctx context.Context, kubeContext, namespace string) ([]string, error)
	Ports(ctx context.Context, kubeContext, namespace, target string) ([]kube.Port, error)
}

// Level is a field that receives suggestions. Each level depends on the ones
// before it.
type Level int

// Levels, in dependency order.
const (
	Namespaces Level = iota
	Targets
	Ports
)

var levels = [...]Level{Namespaces, Targets, Ports}

// Choice is a suggestion: Value goes into the field, Label is shown.
type Choice struct {
	Value, Label string
	// Port is the port number of a Ports choice, 0 otherwise.
	Port int32
}

// Timing of the suggestions.
const (
	// TypingDelay waits for a pause in the typing before querying the cluster.
	TypingDelay = 400 * time.Millisecond
	// QueryTimeout bounds a query to an unresponsive cluster.
	QueryTimeout = 10 * time.Second
)

// Suggester queries the cluster for the suggestions of the fields, as the
// user fills the form. Its methods, and deliver, run on the UI thread, where
// post executes functions: its state needs no lock.
type Suggester struct {
	src     Source
	post    func(func())
	deliver func(Level, []Choice, error)

	kubeContext, namespace, target string
	requests                       [len(levels)]request
}

// request is the latest query of a level; gen identifies it, so that the
// answers of older queries can be recognized and ignored.
type request struct {
	gen    int
	timer  *time.Timer
	cancel context.CancelFunc
}

// NewSuggester returns a suggester; deliver receives the suggestions of a
// level, or why there are none.
func NewSuggester(src Source, post func(func()), deliver func(Level, []Choice, error)) *Suggester {
	return &Suggester{src: src, post: post, deliver: deliver}
}

// SetContext changes the kubeconfig context: every level is queried again,
// at once, since a context is picked in a list rather than typed.
func (s *Suggester) SetContext(kubeContext string) {
	s.kubeContext = kubeContext
	s.refresh(Namespaces, 0)
}

// SetNamespace changes the namespace, once the typing pauses.
func (s *Suggester) SetNamespace(namespace string) {
	s.namespace = namespace
	s.refresh(Targets, TypingDelay)
}

// SetTarget changes the target, once the typing pauses.
func (s *Suggester) SetTarget(target string) {
	s.target = target
	s.refresh(Ports, TypingDelay)
}

// Close cancels the pending queries: the dialog is gone.
func (s *Suggester) Close() {
	for i := range s.requests {
		s.stop(Level(i))
	}
}

// refresh queries from level down to the last one: the choices of a level
// depend on the fields before it.
func (s *Suggester) refresh(from Level, delay time.Duration) {
	for _, l := range levels[from:] {
		s.stop(l)
		r := &s.requests[l]
		r.gen++
		gen := r.gen
		r.timer = time.AfterFunc(delay, func() {
			s.post(func() { s.start(l, gen) })
		})
	}
}

func (s *Suggester) stop(l Level) {
	r := &s.requests[l]
	// A new generation makes any answer on its way stale
	r.gen++
	if r.timer != nil {
		r.timer.Stop()
	}
	if r.cancel != nil {
		r.cancel()
	}
}

// start runs the query of level l, unless a newer one replaced it meanwhile.
func (s *Suggester) start(l Level, gen int) {
	r := &s.requests[l]
	if r.gen != gen {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeout)
	r.cancel = cancel
	kubeContext, namespace, target := s.kubeContext, s.namespace, s.target

	go func() {
		defer cancel()
		choices, err := s.query(ctx, l, kubeContext, namespace, target)
		s.post(func() {
			if s.requests[l].gen == gen {
				s.deliver(l, choices, err)
			}
		})
	}()
}

func (s *Suggester) query(
	ctx context.Context,
	l Level,
	kubeContext, namespace, target string,
) ([]Choice, error) {
	switch l {
	case Namespaces:
		names, err := s.src.Namespaces(ctx, kubeContext)
		return plain(names), err
	case Targets:
		targets, err := s.src.Targets(ctx, kubeContext, namespace)
		return plain(targets), err
	case Ports:
		if target == "" {
			return nil, nil
		}
		ports, err := s.src.Ports(ctx, kubeContext, namespace, target)
		return portChoices(ports), err
	}
	return nil, nil
}

func plain(values []string) []Choice {
	choices := make([]Choice, 0, len(values))
	for _, v := range values {
		choices = append(choices, Choice{Value: v, Label: v})
	}
	return choices
}

// portChoices prefers the name of a port as value: it survives a change of
// its number.
func portChoices(ports []kube.Port) []Choice {
	choices := make([]Choice, 0, len(ports))
	for _, p := range ports {
		number := strconv.Itoa(int(p.Number))
		c := Choice{Value: number, Label: number, Port: p.Number}
		if p.Name != "" {
			c.Value, c.Label = p.Name, p.Name+" · "+number
		}
		choices = append(choices, c)
	}
	return choices
}

// LocalPortFor proposes a local port for a remote one: the same number, the
// most common forward, unless it is privileged. Below 1024 only root may
// listen, so 80 becomes 8080 and 443 becomes 8443, as usual.
func LocalPortFor(remote int32) int32 {
	if remote < 1024 {
		return remote + 8000
	}
	return remote
}
