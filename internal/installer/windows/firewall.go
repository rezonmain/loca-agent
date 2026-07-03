package windows

import (
	"context"
	"strconv"

	"github.com/rezonmain/loca-agent/internal/sys"
)

// firewallRuleName is the netsh rule identifier for the WireGuard listener.
const firewallRuleName = "bootstrap-ai WireGuard"

// addRuleArgs builds the netsh command to allow inbound WireGuard UDP traffic.
func addRuleArgs(name string, port int) []string {
	return []string{
		"advfirewall", "firewall", "add", "rule",
		"name=" + name,
		"dir=in", "action=allow", "protocol=UDP",
		"localport=" + strconv.Itoa(port),
	}
}

func showRuleArgs(name string) []string {
	return []string{"advfirewall", "firewall", "show", "rule", "name=" + name}
}

func deleteRuleArgs(name string) []string {
	return []string{"advfirewall", "firewall", "delete", "rule", "name=" + name}
}

// firewallRuleExists reports whether the named rule is present. netsh exits
// non-zero when no matching rule exists, which we treat as "absent".
func firewallRuleExists(ctx context.Context, run sys.Runner, name string) bool {
	_, err := run.Run(ctx, "netsh", showRuleArgs(name)...)
	return err == nil
}
