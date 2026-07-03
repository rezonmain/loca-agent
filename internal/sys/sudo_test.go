package sys

import (
	"context"
	"reflect"
	"testing"
)

type capture struct{ got []string }

func (c *capture) Run(_ context.Context, name string, args ...string) (string, error) {
	c.got = append([]string{name}, args...)
	return "", nil
}

func TestWithSudoPrefixes(t *testing.T) {
	c := &capture{}
	r := WithSudo(c)
	if _, err := r.Run(context.Background(), "wg-quick", "up", "/etc/wg/wg0.conf"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := []string{"sudo", "wg-quick", "up", "/etc/wg/wg0.conf"}
	if !reflect.DeepEqual(c.got, want) {
		t.Errorf("got %v, want %v", c.got, want)
	}
}
