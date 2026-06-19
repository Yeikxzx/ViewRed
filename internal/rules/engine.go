package rules

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/viewred/viewred/internal/capture"
)

// Rule represents a parsed Suricata-style rule.
type Rule struct {
	Action     string
	Protocol   string
	SrcIP      string
	SrcPort    string
	SrcPortInt int
	DstIP      string
	DstPort    string
	DstPortInt int
	Options    map[string]string
}

// Parse parses a single line of a Suricata-style rule.
// E.g., alert tcp any any -> any 80 (msg:"HTTP traffic"; sid:1;)
func Parse(line string) (*Rule, error) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return nil, errors.New("empty or comment line")
	}

	// Basic split on options
	parts := strings.SplitN(line, "(", 2)
	header := strings.Fields(strings.TrimSpace(parts[0]))
	if len(header) < 7 {
		return nil, fmt.Errorf("invalid rule header: %q", parts[0])
	}

	action := header[0]
	proto := header[1]
	srcIP := header[2]
	srcPort := header[3]
	direction := header[4] // ->
	dstIP := header[5]
	dstPort := header[6]

	if direction != "->" && direction != "<>" {
		return nil, fmt.Errorf("unsupported direction: %s", direction)
	}

	options := make(map[string]string)
	if len(parts) > 1 {
		optsStr := strings.TrimSuffix(strings.TrimSpace(parts[1]), ")")
		optsArr := strings.Split(optsStr, ";")
		for _, opt := range optsArr {
			opt = strings.TrimSpace(opt)
			if opt == "" {
				continue
			}
			optParts := strings.SplitN(opt, ":", 2)
			if len(optParts) == 2 {
				key := strings.TrimSpace(optParts[0])
				val := strings.TrimSpace(optParts[1])
				// remove quotes
				val = strings.TrimPrefix(val, "\"")
				val = strings.TrimSuffix(val, "\"")
				options[key] = val
			} else {
				options[optParts[0]] = ""
			}
		}
	}

	r := &Rule{
		Action:   action,
		Protocol: proto,
		SrcIP:    srcIP,
		SrcPort:  srcPort,
		DstIP:    dstIP,
		DstPort:  dstPort,
		Options:  options,
	}

	if srcPort != "any" {
		if p, err := strconv.Atoi(srcPort); err == nil {
			r.SrcPortInt = p
		}
	}
	if dstPort != "any" {
		if p, err := strconv.Atoi(dstPort); err == nil {
			r.DstPortInt = p
		}
	}

	return r, nil
}

// Engine evaluates packets against a set of rules.
type Engine struct {
	rules []*Rule
}

// NewEngine creates a new rules engine.
func NewEngine() *Engine {
	return &Engine{
		rules: make([]*Rule, 0),
	}
}

// AddRule parses and adds a rule to the engine.
func (e *Engine) AddRule(line string) error {
	r, err := Parse(line)
	if err != nil {
		if err.Error() == "empty or comment line" {
			return nil
		}
		return err
	}
	e.rules = append(e.rules, r)
	return nil
}

// Evaluate checks a packet against all loaded rules and returns a list of matching rules.
func (e *Engine) Evaluate(pkt capture.PacketEvent) []*Rule {
	var matches []*Rule
	for _, r := range e.rules {
		if match(r, pkt) {
			matches = append(matches, r)
		}
	}
	return matches
}

func match(r *Rule, pkt capture.PacketEvent) bool {
	// Protocol check
	if r.Protocol != "any" && !strings.EqualFold(r.Protocol, pkt.Protocol) {
		if r.Protocol == "ip" {
			// IP matches anything with an IP layer, which we assume all our packets have
		} else {
			return false
		}
	}

	// Src check
	if r.SrcIP != "any" && r.SrcIP != pkt.SrcIP {
		// Does not handle CIDR in V1, just exact or 'any'
		return false
	}
	if r.SrcPort != "any" {
		if r.SrcPortInt != int(pkt.SrcPort) {
			return false
		}
	}

	// Dst check
	if r.DstIP != "any" && r.DstIP != pkt.DstIP {
		return false
	}
	if r.DstPort != "any" {
		if r.DstPortInt != int(pkt.DstPort) {
			return false
		}
	}

	return true
}
