package jrsonnet

import "testing"

// An object references the context its fields are evaluated in, which
// references the object, a cycle jrsonnet only frees when collecting cycles.
const cyclicSnippet = `{ a: 1, b: self.a }`

// evaluateTracked evaluates cyclicSnippet n times, in every mode and with
// errors mixed in, and returns the number of tracked objects afterwards.
func evaluateTracked(t *testing.T, vm *VM, n int) int {
	t.Helper()
	for i := range n {
		var err error
		switch i % 4 {
		case 0:
			_, err = vm.EvaluateSnippet("t.jsonnet", cyclicSnippet)
		case 1:
			_, err = vm.EvaluateSnippetMulti("t.jsonnet", "{ a: "+cyclicSnippet+" }")
		case 2:
			_, err = vm.EvaluateSnippetStream("t.jsonnet", "["+cyclicSnippet+"]")
		case 3:
			if _, err := vm.EvaluateSnippet("t.jsonnet", cyclicSnippet+".b + error 'boom'"); err == nil {
				t.Fatal("evaluating an error succeeded")
			}
		}
		if err != nil {
			t.Fatalf("evaluation %d: %v", i, err)
		}
	}
	tracked, err := vm.trackedObjects()
	if err != nil {
		t.Fatal(err)
	}
	return tracked
}

func TestGCInterval(t *testing.T) {
	vm := newTestVM(t, WithGCInterval(10))
	before := evaluateTracked(t, vm, 100)
	after := evaluateTracked(t, vm, 1000)
	t.Logf("tracked objects: %d after 100 evaluations, %d after 1100", before, after)
	if after > before {
		t.Errorf("with WithGCInterval(10), tracked objects grew from %d to %d", before, after)
	}
}

func TestGCIntervalDisabledByDefault(t *testing.T) {
	vm := newTestVM(t)
	before := evaluateTracked(t, vm, 100)
	after := evaluateTracked(t, vm, 1000)
	t.Logf("tracked objects: %d after 100 evaluations, %d after 1100", before, after)
	if after < before+1000 {
		t.Errorf("without WithGCInterval, tracked objects went from %d to %d, want them to grow by at least 1000", before, after)
	}
}
