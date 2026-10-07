package jrsonnet

import (
	"errors"
	"strings"
)

// Option configures a VM during creation.
type Option interface {
	apply(c *config) error
}

type optionFunc func(c *config) error

func (f optionFunc) apply(c *config) error { return f(c) }

// config holds the options. New creates the VM from it once all options
// have run.
type config struct {
	maxStack        *uint
	maxTrace        *uint
	stringOutput    bool
	trailingNewline bool
	jpaths          []string
	importer        ImportFunc
	gcInterval      uint
}

func newConfig(opts []Option) (*config, error) {
	c := &config{trailingNewline: true}
	for _, opt := range opts {
		if err := opt.apply(c); err != nil {
			return nil, err
		}
	}
	if c.importer != nil && len(c.jpaths) > 0 {
		return nil, errors.New("jrsonnet: WithJPath and WithImporter can't be used together")
	}
	return c, nil
}

// WithMaxStack sets the maximum stack depth for the VM. Defaults to 500, as in
// go-jsonnet.
//
// Past a few thousand frames, jrsonnet continues on a stack it allocates
// itself. A native function or importer called from there crashes the
// process, so keep the limit well below that when using either.
func WithMaxStack(depth uint) Option {
	return optionFunc(func(c *config) error {
		c.maxStack = &depth
		return nil
	})
}

// WithMaxTrace sets the maximum number of stack frames shown in an error,
// keeping the first and last ones. As in go-jsonnet, it defaults to 20, and 0
// shows all.
func WithMaxTrace(frames uint) Option {
	return optionFunc(func(c *config) error {
		c.maxTrace = &frames
		return nil
	})
}

// WithStringOutput sets whether the output should be treated as a raw string
// (true) or JSON (false). As in go-jsonnet, with string output a result or
// multi field that isn't a string is an error. Unlike go-jsonnet, this also
// applies to stream elements.
func WithStringOutput(enabled bool) Option {
	return optionFunc(func(c *config) error {
		c.stringOutput = enabled
		return nil
	})
}

// WithTrailingNewline sets whether evaluation output should have a trailing
// newline appended. Defaults to true, matching go-jsonnet and upstream jrsonnet.
func WithTrailingNewline(enabled bool) Option {
	return optionFunc(func(c *config) error {
		c.trailingNewline = enabled
		return nil
	})
}

// WithJPath adds library search paths for import resolution. As in
// go-jsonnet, a file in a later path wins. Paths are used byte for byte, and
// one containing a NUL byte makes New fail.
//
// WithJPath can't be combined with WithImporter.
func WithJPath(paths ...string) Option {
	return optionFunc(func(c *config) error {
		for _, p := range paths {
			if strings.IndexByte(p, 0) >= 0 {
				return errors.New("jrsonnet: jpath contains a NUL byte: " + p)
			}
		}
		c.jpaths = append(c.jpaths, paths...)
		return nil
	})
}

// WithGCInterval makes the VM collect reference cycles after every n
// evaluations, successful or not. 0, the default, disables it.
//
// jrsonnet frees most values by reference counting, but never collects
// cycles, such as those of local functions, by itself. Without collecting
// them, a VM that's evaluated many times keeps growing until Close. The cost
// of a collection grows with the number of objects alive on the VM, which
// includes the values cached for evaluated files.
func WithGCInterval(n uint) Option {
	return optionFunc(func(c *config) error {
		c.gcInterval = n
		return nil
	})
}
