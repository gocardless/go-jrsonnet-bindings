package jrsonnet

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
