package rules

// DefaultRuleStrings returns the built-in SSH/Telnet detection rules used when
// no custom rules are configured or when storage is unavailable. The rules are
// SYN-only to suppress noise from ongoing sessions (ACKs, keepalives, data).
//
// Keeping the rule text in this package lets the rules engine test the default
// copy, severity, flags, and SYN-only behavior directly, without depending on
// the binary's main package.
func DefaultRuleStrings() []string {
	return []string{
		`alert tcp any any -> any 22 (msg:"SSH Connection Attempt"; severity:INFO; flags:S;)`,
		`alert tcp any any -> any 23 (msg:"Insecure Telnet Traffic"; severity:WARNING; flags:S;)`,
	}
}
