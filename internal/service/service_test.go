package service

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// fakeRunner records every invocation and returns a programmable response.
type fakeRunner struct {
	calls   [][]string
	handler func(name string, args []string) (string, error)
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	if f.handler != nil {
		return f.handler(name, args)
	}
	return "", nil
}

func (f *fakeRunner) hasCall(want []string) bool {
	for _, c := range f.calls {
		if reflect.DeepEqual(c, want) {
			return true
		}
	}
	return false
}

func TestInstallBuildsExpectedCommands(t *testing.T) {
	r := &fakeRunner{}
	m := NewNSSM("nssm.exe", r)

	spec := Spec{
		Name:        "svc",
		DisplayName: "Display",
		Executable:  `C:\llama\llama-server.exe`,
		Args:        []string{"--port", "8080"},
		WorkingDir:  `C:\llama`,
		Stdout:      `C:\logs\out.log`,
		Env:         map[string]string{"B": "2", "A": "1"},
		AutoStart:   true,
	}
	if err := m.Install(context.Background(), spec); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// First call installs the program with its args.
	want0 := []string{"nssm.exe", "install", "svc", `C:\llama\llama-server.exe`, "--port", "8080"}
	if !reflect.DeepEqual(r.calls[0], want0) {
		t.Errorf("install call = %v, want %v", r.calls[0], want0)
	}

	checks := [][]string{
		{"nssm.exe", "set", "svc", "DisplayName", "Display"},
		{"nssm.exe", "set", "svc", "AppDirectory", `C:\llama`},
		{"nssm.exe", "set", "svc", "AppStdout", `C:\logs\out.log`},
		// Env keys are sorted for determinism.
		{"nssm.exe", "set", "svc", "AppEnvironmentExtra", "A=1", "B=2"},
		{"nssm.exe", "set", "svc", "Start", "SERVICE_AUTO_START"},
	}
	for _, c := range checks {
		if !r.hasCall(c) {
			t.Errorf("expected nssm call %v, calls were:\n%v", c, r.calls)
		}
	}
}

func TestInstallDemandStartWhenNotAutoStart(t *testing.T) {
	r := &fakeRunner{}
	m := NewNSSM("nssm", r)
	if err := m.Install(context.Background(), Spec{Name: "s", Executable: "x"}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !r.hasCall([]string{"nssm", "set", "s", "Start", "SERVICE_DEMAND_START"}) {
		t.Errorf("expected demand-start; calls: %v", r.calls)
	}
}

func TestRemoveStopsThenRemoves(t *testing.T) {
	r := &fakeRunner{}
	m := NewNSSM("nssm", r)
	if err := m.Remove(context.Background(), "svc"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(r.calls) != 2 ||
		!reflect.DeepEqual(r.calls[0], []string{"nssm", "stop", "svc"}) ||
		!reflect.DeepEqual(r.calls[1], []string{"nssm", "remove", "svc", "confirm"}) {
		t.Errorf("unexpected remove sequence: %v", r.calls)
	}
}

func TestStatusParsing(t *testing.T) {
	tests := []struct {
		name    string
		out     string
		err     error
		want    State
		wantErr bool
	}{
		{"running", "SERVICE_RUNNING", nil, StateRunning, false},
		{"stopped", "SERVICE_STOPPED", nil, StateStopped, false},
		{"missing", "Can't open service! ...", errors.New("exit 1"), StateNotInstalled, false},
		{"other error", "weird", errors.New("nssm exploded"), StateUnknown, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &fakeRunner{handler: func(_ string, _ []string) (string, error) {
				return tc.out, tc.err
			}}
			m := NewNSSM("nssm", r)
			got, err := m.Status(context.Background(), "svc")
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("state = %q, want %q", got, tc.want)
			}
		})
	}
}
