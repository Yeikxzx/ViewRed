package rules

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

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
	rules   []*Rule
	rulesMu sync.RWMutex
	dedupMu sync.Mutex
	dedup   map[string]time.Time // fingerprint -> when it was last alerted
	dedupTTL time.Duration
}

// NewEngine creates a new rules engine with a 5-minute dedup interval.
func NewEngine() *Engine {
	return &Engine{
		rules:    make([]*Rule, 0),
		dedup:    make(map[string]time.Time),
		dedupTTL: 5 * time.Minute,
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
	e.rulesMu.Lock()
	e.rules = append(e.rules, r)
	e.rulesMu.Unlock()
	return nil
}

// RemoveRule removes a rule by its SID (from options["sid"]).
// Returns false if no rule matched. Thread-safe via rulesMu.
func (e *Engine) RemoveRule(sid string) bool {
	e.rulesMu.Lock()
	defer e.rulesMu.Unlock()

	for i, r := range e.rules {
		if r.Options["sid"] == sid {
			// Remove from slice
			e.rules = append(e.rules[:i], e.rules[i+1:]...)
			return true
		}
	}
	return false
}

// Evaluate checks a packet against all loaded rules and returns matching
// rules that haven't fired in the last dedupTTL (5 min) for the same
// (rule + src + dst + dst_port) tuple.
func (e *Engine) Evaluate(pkt capture.PacketEvent) []*Rule {
	var matches []*Rule
	now := time.Now()

	// Garbage-collect expired dedup entries every 100 calls (cheap, no
	// goroutine needed for a 5-min TTL).
	if len(e.dedup) > 1000 {
		e.dedupMu.Lock()
		for fp, last := range e.dedup {
			if now.Sub(last) > e.dedupTTL {
				delete(e.dedup, fp)
			}
		}
		e.dedupMu.Unlock()
	}

	e.rulesMu.RLock()
	defer e.rulesMu.RUnlock()

	for _, r := range e.rules {
		if !match(r, pkt) {
			continue
		}

		// Dedup key: rule message + src IP + dst IP + dst port.
		fp := fmt.Sprintf("%s|%s|%s|%s", r.Options["msg"], pkt.SrcIP, pkt.DstIP, r.DstPort)

		e.dedupMu.Lock()
		lastAlert, seen := e.dedup[fp]
		if seen && now.Sub(lastAlert) < e.dedupTTL {
			// Already alerted recently — skip.
			e.dedupMu.Unlock()
			continue
		}
		e.dedup[fp] = now
		e.dedupMu.Unlock()

		matches = append(matches, r)
	}
	return matches
}

func match(r *Rule, pkt capture.PacketEvent) bool {
	// Graceful skip for advanced options we don't fully support in V1
	// to avoid false positives on generic rules that require them.
	if _, ok := r.Options["flags"]; ok {
		return false
	}
	if _, ok := r.Options["threshold"]; ok {
		return false
	}
	if _, ok := r.Options["flowbits"]; ok {
		return false
	}

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
