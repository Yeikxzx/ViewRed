package scanner

import (
	"context"
	"encoding/xml"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/viewred/viewred/internal/storage"
)

// NmapRun corresponds to the root XML element of nmap output.
type NmapRun struct {
	XMLName xml.Name `xml:"nmaprun"`
	Hosts   []Host   `xml:"host"`
}

type Host struct {
	Status Status `xml:"status"`
	Ports  Ports  `xml:"ports"`
}

type Status struct {
	State string `xml:"state,attr"`
}

type Ports struct {
	Ports []Port `xml:"port"`
}

type Port struct {
	PortID  int     `xml:"portid,attr"`
	State   State   `xml:"state"`
	Service Service `xml:"service"`
	Scripts []Script `xml:"script"`
}

type State struct {
	State string `xml:"state,attr"`
}

type Service struct {
	Name    string `xml:"name,attr"`
	Product string `xml:"product,attr"`
	Version string `xml:"version,attr"`
}

type Script struct {
	ID     string `xml:"id,attr"`
	Output string `xml:"output,attr"`
	Tables []Table `xml:"table"`
}

type Table struct {
	Key      string `xml:"key,attr"`
	Elements []Elem `xml:"elem"`
	Tables   []Table `xml:"table"`
}

type Elem struct {
	Key   string `xml:"key,attr"`
	Value string `xml:",chardata"`
}

// Engine wraps the nmap binary invocation.
type Engine struct {
	querier storage.Querier
}

// NewEngine creates a new scanner engine.
func NewEngine(q storage.Querier) *Engine {
	return &Engine{querier: q}
}

// Scan synchronously executes an nmap scan and persists the findings.
// Profile can be "quick", "full", or "service-detect".
func (e *Engine) Scan(ctx context.Context, ip string, profile string) error {
	// Build nmap args
	args := []string{"-oX", "-"} // Output XML to stdout
	
	switch profile {
	case "quick":
		args = append(args, "--top-ports", "1000")
	case "full":
		args = append(args, "-p", "1-65535")
	case "service-detect":
		args = append(args, "-sV", "--script=vulners")
	default:
		args = append(args, "--top-ports", "100") // fallback safe
	}
	
	args = append(args, ip)

	cmd := exec.CommandContext(ctx, "nmap", args...)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("nmap execution failed: %w", err)
	}

	var run NmapRun
	if err := xml.Unmarshal(out, &run); err != nil {
		return fmt.Errorf("failed to parse nmap xml: %w", err)
	}

	for _, host := range run.Hosts {
		if host.Status.State != "up" {
			continue
		}
		
		for _, p := range host.Ports.Ports {
			if p.State.State != "open" {
				continue
			}

			// Base port info
			portNum := p.PortID
			desc := fmt.Sprintf("Open Port %d: %s %s %s", portNum, p.Service.Name, p.Service.Product, p.Service.Version)
			desc = strings.TrimSpace(desc)

			// If we have vulners script output, parse it
			foundVuln := false
			for _, script := range p.Scripts {
				if script.ID == "vulners" {
					for _, table := range script.Tables {
						// parse vulners structure
						var cveID string
						var cvss float64
						var typeVal string

						for _, elem := range table.Elements {
							if elem.Key == "id" {
								cveID = elem.Value
							} else if elem.Key == "cvss" {
								cvss, _ = strconv.ParseFloat(elem.Value, 64)
							} else if elem.Key == "type" {
								typeVal = elem.Value
							}
						}
						
						if typeVal == "cve" && cveID != "" {
							foundVuln = true
							cveRef := cveID
							v := storage.Vulnerability{
								DeviceIP:    ip,
								CVEID:       &cveRef,
								CVSSScore:   &cvss,
								Description: fmt.Sprintf("CVE Found: %s on port %d (%s)", cveID, portNum, p.Service.Product),
								Port:        &portNum,
								Mitigated:   false,
							}
							_, _ = e.querier.SaveVulnerability(ctx, v)
						}
					}
				}
			}

			// If no specific CVE was found, we just log the open port as an "info" vulnerability.
			// This matches the simplified spec where we want to know what's open.
			if !foundVuln {
				v := storage.Vulnerability{
					DeviceIP:    ip,
					Description: desc,
					Port:        &portNum,
					Mitigated:   false,
				}
				_, _ = e.querier.SaveVulnerability(ctx, v)
			}
		}
	}

	return nil
}
