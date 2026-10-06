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

// WithMaxStack sets the maximum stack depth for the VM.
func WithMaxStack(depth uint) Option {
	return optionFunc(func(c *config) error {
		c.maxStack = &depth
		return nil
	})
}

// WithMaxTrace sets the maximum number of stack trace lines displayed on error.
// 0 means show all.
func WithMaxTrace(lines uint) Option {
	return optionFunc(func(c *config) error {
		c.maxTrace = &lines
		return nil
	})
}

// WithStringOutput sets whether the output should be treated as a raw string
// (true) or JSON (false).
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
