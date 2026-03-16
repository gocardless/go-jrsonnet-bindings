package jrsonnet

// EvaluateSnippetMulti evaluates a Jsonnet snippet expecting multi-file output.
// Not yet implemented.
func (v *VM) EvaluateSnippetMulti(filename, snippet string) (map[string]string, error) {
	return nil, ErrNotImplemented
}

// EvaluateFileMulti evaluates a Jsonnet file expecting multi-file output.
// Not yet implemented.
func (v *VM) EvaluateFileMulti(filename string) (map[string]string, error) {
	return nil, ErrNotImplemented
}

// EvaluateSnippetStream evaluates a Jsonnet snippet expecting stream output.
// Not yet implemented.
func (v *VM) EvaluateSnippetStream(filename, snippet string) ([]string, error) {
	return nil, ErrNotImplemented
}

// EvaluateFileStream evaluates a Jsonnet file expecting stream output.
// Not yet implemented.
func (v *VM) EvaluateFileStream(filename string) ([]string, error) {
	return nil, ErrNotImplemented
}
