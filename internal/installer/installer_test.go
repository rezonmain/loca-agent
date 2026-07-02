package installer

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// mkStep returns a FuncStep that appends "phase:name" events to a shared log,
// letting tests assert exactly which callbacks fired and in what order.
func mkStep(events *[]string, name string, satisfied bool, verifyErr, runErr error) Step {
	return FuncStep{
		Label: name,
		VerifyFunc: func(_ context.Context, _ *State) (bool, error) {
			*events = append(*events, "verify:"+name)
			return satisfied, verifyErr
		},
		RunFunc: func(_ context.Context, _ *State) error {
			*events = append(*events, "run:"+name)
			return runErr
		},
		RollbackFunc: func(_ context.Context, _ *State) error {
			*events = append(*events, "rollback:"+name)
			return nil
		},
	}
}

func assertSeq(t *testing.T, got []string, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("event sequence mismatch\n got: %v\nwant: %v", got, want)
	}
}

func TestEngineRunsInOrder(t *testing.T) {
	var ev []string
	steps := []Step{
		mkStep(&ev, "a", false, nil, nil),
		mkStep(&ev, "b", false, nil, nil),
	}
	if err := (&Engine{}).Run(context.Background(), NewState(MacClient, nil), steps); err != nil {
		t.Fatalf("Run: %v", err)
	}
	assertSeq(t, ev, []string{"verify:a", "run:a", "verify:b", "run:b"})
}

func TestEngineSkipsSatisfiedSteps(t *testing.T) {
	var ev []string
	steps := []Step{
		mkStep(&ev, "a", true, nil, nil), // already satisfied → Run skipped
		mkStep(&ev, "b", false, nil, nil),
	}
	if err := (&Engine{}).Run(context.Background(), NewState(MacClient, nil), steps); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Resume semantics: satisfied step verified but not run.
	assertSeq(t, ev, []string{"verify:a", "verify:b", "run:b"})
}

func TestEngineRollsBackCompletedOnFailure(t *testing.T) {
	var ev []string
	boom := errors.New("boom")
	steps := []Step{
		mkStep(&ev, "a", false, nil, nil),
		mkStep(&ev, "b", false, nil, boom), // fails during Run
		mkStep(&ev, "c", false, nil, nil),  // never reached
	}
	err := (&Engine{}).Run(context.Background(), NewState(MacClient, nil), steps)
	if !errors.Is(err, boom) {
		t.Fatalf("expected boom, got %v", err)
	}
	// a completed → rolled back; b failed mid-Run → not rolled back; c untouched.
	assertSeq(t, ev, []string{"verify:a", "run:a", "verify:b", "run:b", "rollback:a"})
}

func TestEngineRunErrorSurfacesUnwrapped(t *testing.T) {
	var ev []string
	boom := errors.New("original cause")
	steps := []Step{mkStep(&ev, "a", false, nil, boom)}
	err := (&Engine{}).Run(context.Background(), NewState(MacClient, nil), steps)
	// Run errors are returned as-is (not wrapped) so an actionable error surfaces.
	if err != boom {
		t.Errorf("expected the exact run error to surface, got %v", err)
	}
}

func TestEngineVerifyErrorAborts(t *testing.T) {
	var ev []string
	verr := errors.New("cannot verify")
	steps := []Step{
		mkStep(&ev, "a", false, nil, nil),
		mkStep(&ev, "b", false, verr, nil), // Verify errors
		mkStep(&ev, "c", false, nil, nil),
	}
	err := (&Engine{}).Run(context.Background(), NewState(MacClient, nil), steps)
	if !errors.Is(err, verr) {
		t.Fatalf("expected verify error, got %v", err)
	}
	assertSeq(t, ev, []string{"verify:a", "run:a", "verify:b", "rollback:a"})
}

func TestFuncStepDefaults(t *testing.T) {
	// A FuncStep with nil funcs: Verify=false (runs), Run=no-op, Rollback=no-op.
	s := FuncStep{Label: "empty"}
	sat, err := s.Verify(context.Background(), nil)
	if sat || err != nil {
		t.Errorf("default Verify = (%v,%v), want (false,nil)", sat, err)
	}
	if err := s.Run(context.Background(), nil); err != nil {
		t.Errorf("default Run err = %v", err)
	}
	if err := s.Rollback(context.Background(), nil); err != nil {
		t.Errorf("default Rollback err = %v", err)
	}
}

func TestStateData(t *testing.T) {
	st := NewState(WindowsServer, nil)
	st.Set("token", "abc")
	if st.String("token") != "abc" {
		t.Errorf("String(token) = %q", st.String("token"))
	}
	if _, ok := st.Get("missing"); ok {
		t.Errorf("Get(missing) should be absent")
	}
	if st.String("missing") != "" {
		t.Errorf("String(missing) should be empty")
	}
}

func TestParseMachineKind(t *testing.T) {
	cases := map[string]MachineKind{
		"client": MacClient, "mac": MacClient, "macos-client": MacClient,
		"server": WindowsServer, "windows": WindowsServer,
	}
	for in, want := range cases {
		got, err := ParseMachineKind(in)
		if err != nil || got != want {
			t.Errorf("ParseMachineKind(%q) = (%v,%v), want %v", in, got, err, want)
		}
	}
	if _, err := ParseMachineKind("frobnicate"); err == nil {
		t.Errorf("expected error for invalid role")
	}
}
