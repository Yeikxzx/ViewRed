package reports

import (
	"fmt"
	"time"
	"strings"
	"encoding/json"
	"net"

	"github.com/go-pdf/fpdf"
	"github.com/viewred/viewred/internal/storage"
)

func generatePDF(
	filePath string, 
	since time.Time, 
	until time.Time, 
	testContext *storage.ReportHistory,
	summary storage.DashboardSummary, 
	topTalkers []storage.TopTalker, 
	protocols []storage.ProtocolShare, 
	topPorts []storage.TopPort,
	alerts []storage.Alert,
	timeline []storage.TrafficPoint,
	topConvs []storage.TopConversation,
	devices []storage.Device,
	vulns []storage.Vulnerability,
) error {
	pdf := fpdf.New("P", "mm", "A4", "")
	
	// Set header and footer
	pdf.SetHeaderFunc(func() {
		pdf.SetFont("Arial", "B", 10)
		pdf.SetTextColor(128, 128, 128)
		pdf.CellFormat(0, 10, "ViewRed NDR - Reporte Confidencial", "", 0, "L", false, 0, "")
		pdf.Ln(10)
	})
	
	pdf.SetFooterFunc(func() {
		pdf.SetY(-15)
		pdf.SetFont("Arial", "I", 8)
		pdf.SetTextColor(128, 128, 128)
		pdf.CellFormat(0, 10, fmt.Sprintf("Generado el: %s | Pagina %d", time.Now().Format("2006-01-02 15:04:05"), pdf.PageNo()), "", 0, "C", false, 0, "")
	})

	pdf.AddPage()

	// Colors
	primaryColor := []int{211, 47, 47} // Red
	darkGray := []int{51, 51, 51}
	lightGray := []int{240, 240, 240}

	// Title
	pdf.SetFont("Arial", "B", 24)
	pdf.SetTextColor(primaryColor[0], primaryColor[1], primaryColor[2])
	pdf.CellFormat(0, 15, "ViewRed - Reporte de Seguridad de Red", "", 1, "C", false, 0, "")
	
	// 1. Datos generales del reporte
	addSectionTitle(pdf, "Datos generales del reporte")
	pdf.SetFont("Arial", "", 11)
	pdf.SetFillColor(lightGray[0], lightGray[1], lightGray[2])
	
	addTableRow(pdf, "Periodo analizado:", fmt.Sprintf("%s a %s", since.Format("2006-01-02 15:04"), until.Format("2006-01-02 15:04")), true)
	addTableRow(pdf, "Sensor / Red:", "ViewRed-01 (Interfaz principal)", false)
	addTableRow(pdf, "Modo de captura:", "Promiscuo", true)
	pdf.Ln(5)

	// 2. Resumen ejecutivo
	addSectionTitle(pdf, "Resumen ejecutivo")
	totalBytesSec := formatBytes(summary.BytesPerSec) + "/s"
	addTableRow(pdf, "Total Dispositivos:", fmt.Sprintf("%d", summary.DeviceCount), true)
	addTableRow(pdf, "Flujos observados:", fmt.Sprintf("%d", summary.ActiveConnections), false)
	addTableRow(pdf, "Trafico promedio (p/s):", fmt.Sprintf("%.2f p/s", summary.PacketsPerSec), true)
	addTableRow(pdf, "Ancho de banda prom.:", totalBytesSec, false)
	addTableRow(pdf, "Alertas detectadas:", fmt.Sprintf("%d", len(alerts)), true)
	
	trafficRisk := "BAJO"
	if len(alerts) > 5 { trafficRisk = "MEDIO" }
	if len(alerts) > 15 { trafficRisk = "ALTO" }
	
	exposureRisk := "BAJO"
	if len(vulns) > 2 { exposureRisk = "MEDIO" }
	if len(vulns) > 5 { exposureRisk = "ALTO" }
	for _, v := range vulns {
		if v.CVEID != nil && strings.Contains(strings.ToLower(*v.CVEID), "cve") {
			exposureRisk = "ALTO"
			break
		}
	}
	
	coverageRisk := "CONFIABLE" // Assuming if we are generating a report, basic engines are up. Could be enhanced.
	
	addTableRow(pdf, "Riesgo de Trafico/Deteccion:", trafficRisk, false)
	addTableRow(pdf, "Riesgo de Exposicion (Nmap):", exposureRisk, true)
	addTableRow(pdf, "Riesgo de Cobertura/Visibilidad:", coverageRisk, false)
	pdf.Ln(5)

	// 3. Contexto de la prueba
	if testContext != nil && testContext.ScenarioName != "" {
		addSectionTitle(pdf, "Contexto de la prueba")
		addTableRow(pdf, "Escenario:", testContext.ScenarioName, true)
		addTableRow(pdf, "Equipo Victima:", testContext.VictimIP, false)
		addTableRow(pdf, "Equipo Atacante:", testContext.AttackerIP, true)
		addTableRow(pdf, "Resultado Esperado:", testContext.ExpectedResult, false)
		if testContext.Notes != "" {
			addTableRow(pdf, "Notas adicionales:", testContext.Notes, true)
		}
		pdf.Ln(5)
	}

	// 4. Trafico observado
	addSectionTitle(pdf, "Trafico observado (Linea de tiempo y Top)")
	
	// Render simple timeline chart (text-based or basic bars)
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 8, "Evolucion del trafico (Bytes por minuto):", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 9)
	if len(timeline) == 0 {
		pdf.CellFormat(0, 6, "No hay datos de linea de tiempo suficientes.", "", 1, "L", false, 0, "")
	} else {
		maxBytes := int64(0)
		for _, p := range timeline {
			if p.Bytes > maxBytes {
				maxBytes = p.Bytes
			}
		}
		// Draw a mini bar chart
		startX := pdf.GetX()
		startY := pdf.GetY()
		chartWidth := 150.0
		chartHeight := 20.0
		if maxBytes > 0 {
			stepX := chartWidth / float64(len(timeline))
			pdf.SetFillColor(primaryColor[0], primaryColor[1], primaryColor[2])
			for i, p := range timeline {
				h := (float64(p.Bytes) / float64(maxBytes)) * chartHeight
				x := startX + float64(i)*stepX
				y := startY + chartHeight - h
				if h > 0 {
					pdf.Rect(x, y, stepX-0.5, h, "F")
				}
			}
			pdf.SetY(startY + chartHeight + 5)
			pdf.CellFormat(0, 6, fmt.Sprintf("Pico maximo: %s/min", formatBytesFloat(float64(maxBytes))), "", 1, "L", false, 0, "")
		}
	}
	pdf.Ln(5)

	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 8, "Top Conversaciones:", "", 1, "L", false, 0, "")
	if len(topConvs) == 0 {
		pdf.SetFont("Arial", "", 9)
		pdf.CellFormat(0, 6, "No hay conversaciones registradas.", "", 1, "L", false, 0, "")
	} else {
		pdf.SetFont("Arial", "B", 9)
		pdf.SetFillColor(darkGray[0], darkGray[1], darkGray[2])
		pdf.SetTextColor(255, 255, 255)
		pdf.CellFormat(40, 7, "Origen", "1", 0, "C", true, 0, "")
		pdf.CellFormat(40, 7, "Destino", "1", 0, "C", true, 0, "")
		pdf.CellFormat(20, 7, "Puerto", "1", 0, "C", true, 0, "")
		pdf.CellFormat(20, 7, "Proto", "1", 0, "C", true, 0, "")
		pdf.CellFormat(30, 7, "Bytes", "1", 0, "C", true, 0, "")
		pdf.CellFormat(30, 7, "Paquetes", "1", 1, "C", true, 0, "")
		
		truncateText := func(text string, maxWidth float64) string {
			if pdf.GetStringWidth(text) <= maxWidth {
				return text
			}
			for len(text) > 0 && pdf.GetStringWidth(text+"...") > maxWidth {
				text = text[:len(text)-1]
			}
			return text + "..."
		}

		pdf.SetFont("Arial", "", 9)
		pdf.SetTextColor(0, 0, 0)
		for i, c := range topConvs {
			fill := i%2 == 0
			pdf.SetFillColor(lightGray[0], lightGray[1], lightGray[2])
			pdf.CellFormat(40, 7, truncateText(c.SrcIP, 38), "1", 0, "C", fill, 0, "")
			pdf.CellFormat(40, 7, truncateText(c.DstIP, 38), "1", 0, "C", fill, 0, "")
			pdf.CellFormat(20, 7, fmt.Sprintf("%d", c.DstPort), "1", 0, "C", fill, 0, "")
			pdf.CellFormat(20, 7, c.Protocol, "1", 0, "C", fill, 0, "")
			pdf.CellFormat(30, 7, formatBytesFloat(float64(c.Bytes)), "1", 0, "C", fill, 0, "")
			pdf.CellFormat(30, 7, fmt.Sprintf("%d", c.Packets), "1", 1, "C", fill, 0, "")
		}
	}
	pdf.Ln(5)

	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 8, "Distribucion de Protocolos:", "", 1, "L", false, 0, "")
	if len(protocols) == 0 {
		pdf.SetFont("Arial", "", 9)
		pdf.CellFormat(0, 6, "No hay datos de protocolos.", "", 1, "L", false, 0, "")
	} else {
		pdf.SetFont("Arial", "B", 9)
		pdf.SetFillColor(darkGray[0], darkGray[1], darkGray[2])
		pdf.SetTextColor(255, 255, 255)
		pdf.CellFormat(90, 7, "Protocolo", "1", 0, "C", true, 0, "")
		pdf.CellFormat(90, 7, "Porcentaje (%)", "1", 1, "C", true, 0, "")
		
		pdf.SetFont("Arial", "", 9)
		pdf.SetTextColor(0, 0, 0)
		for i, p := range protocols {
			fill := i%2 == 0
			pdf.SetFillColor(lightGray[0], lightGray[1], lightGray[2])
			pdf.CellFormat(90, 7, p.Protocol, "1", 0, "C", fill, 0, "")
			pdf.CellFormat(90, 7, fmt.Sprintf("%.2f%%", p.Percentage), "1", 1, "C", fill, 0, "")
		}
	}
	pdf.Ln(5)

	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 8, "Top Puertos:", "", 1, "L", false, 0, "")
	if len(topPorts) == 0 {
		pdf.SetFont("Arial", "", 9)
		pdf.CellFormat(0, 6, "No hay datos de puertos.", "", 1, "L", false, 0, "")
	} else {
		pdf.SetFont("Arial", "B", 9)
		pdf.SetFillColor(darkGray[0], darkGray[1], darkGray[2])
		pdf.SetTextColor(255, 255, 255)
		pdf.CellFormat(60, 7, "Puerto Destino", "1", 0, "C", true, 0, "")
		pdf.CellFormat(60, 7, "Bytes", "1", 0, "C", true, 0, "")
		pdf.CellFormat(60, 7, "Paquetes", "1", 1, "C", true, 0, "")
		
		pdf.SetFont("Arial", "", 9)
		pdf.SetTextColor(0, 0, 0)
		for i, p := range topPorts {
			fill := i%2 == 0
			pdf.SetFillColor(lightGray[0], lightGray[1], lightGray[2])
			pdf.CellFormat(60, 7, fmt.Sprintf("%d", p.Port), "1", 0, "C", fill, 0, "")
			pdf.CellFormat(60, 7, formatBytesFloat(float64(p.Bytes)), "1", 0, "C", fill, 0, "")
			pdf.CellFormat(60, 7, fmt.Sprintf("%d", p.Packets), "1", 1, "C", fill, 0, "")
		}
	}
	pdf.Ln(5)

	// 5. Inventario de dispositivos
	addSectionTitle(pdf, "Inventario de dispositivos")
	if len(devices) == 0 {
		pdf.CellFormat(0, 8, "No se registraron dispositivos.", "", 1, "L", false, 0, "")
	} else {
		var localDevices []storage.Device
		var externalDevices []storage.Device

		for _, d := range devices {
			ipStr := d.IP
			// Extract IP from CIDR if present
			if idx := strings.IndexByte(ipStr, '/'); idx != -1 {
				ipStr = ipStr[:idx]
			}
			parsedIP := net.ParseIP(ipStr)
			if parsedIP != nil && parsedIP.IsPrivate() {
				localDevices = append(localDevices, d)
			} else {
				externalDevices = append(externalDevices, d)
			}
		}

		renderDeviceTable := func(title string, devs []storage.Device, isExternal bool) {
			if len(devs) > 0 && pdf.GetY() > 200 {
				pdf.AddPage() // Avoid splitting the table without headers
			}
			pdf.SetFont("Arial", "B", 10)
			pdf.CellFormat(0, 8, title, "", 1, "L", false, 0, "")
			if len(devs) == 0 {
				pdf.SetFont("Arial", "", 9)
				pdf.CellFormat(0, 6, "No hay dispositivos en esta categoria.", "", 1, "L", false, 0, "")
				pdf.Ln(2)
				return
			}
			pdf.SetFont("Arial", "B", 9)
			pdf.SetFillColor(darkGray[0], darkGray[1], darkGray[2])
			pdf.SetTextColor(255, 255, 255)
			pdf.CellFormat(40, 7, "IP", "1", 0, "C", true, 0, "")
			pdf.CellFormat(40, 7, "MAC", "1", 0, "C", true, 0, "")
			pdf.CellFormat(40, 7, "Fabricante", "1", 0, "C", true, 0, "")
			pdf.CellFormat(60, 7, "Hostname", "1", 1, "C", true, 0, "")
			
			truncateText := func(text string, maxWidth float64) string {
				if pdf.GetStringWidth(text) <= maxWidth {
					return text
				}
				for len(text) > 0 && pdf.GetStringWidth(text+"...") > maxWidth {
					text = text[:len(text)-1]
				}
				return text + "..."
			}

			pdf.SetFont("Arial", "", 9)
			pdf.SetTextColor(0, 0, 0)
			for i, d := range devs {
				if i >= 10 { break } // Only show top 10 per category
				fill := i%2 == 0
				pdf.SetFillColor(lightGray[0], lightGray[1], lightGray[2])
				
				displayIP := d.IP
				if idx := strings.IndexByte(displayIP, '/'); idx != -1 {
					displayIP = displayIP[:idx]
				}
				pdf.CellFormat(40, 7, truncateText(displayIP, 38), "1", 0, "C", fill, 0, "")
				
				mac := d.MAC; if mac == "" || isExternal { mac = "-" }
				pdf.CellFormat(40, 7, truncateText(mac, 38), "1", 0, "C", fill, 0, "")
				
				vendor := d.Vendor; if vendor == "" || isExternal { vendor = "-" }
				pdf.CellFormat(40, 7, truncateText(vendor, 38), "1", 0, "C", fill, 0, "")
				
				hostname := d.Hostname; if hostname == "" { hostname = "-" }
				pdf.CellFormat(60, 7, truncateText(hostname, 58), "1", 1, "C", fill, 0, "")
			}
			pdf.Ln(5)
		}
		
		renderDeviceTable("Dispositivos Locales:", localDevices, false)
		renderDeviceTable("Hosts Externos:", externalDevices, true)
	}

	// Separate Alerts (Suricata vs ML)
	var suricataAlerts []storage.Alert
	var mlAlerts []storage.Alert
	for _, a := range alerts {
		if strings.ToLower(a.Source) == "ml" {
			mlAlerts = append(mlAlerts, a)
		} else {
			suricataAlerts = append(suricataAlerts, a)
		}
	}

	// 6. Alertas de Seguridad (Reglas/Custom)
	addSectionTitle(pdf, "Alertas de Seguridad (Reglas)")
	if len(suricataAlerts) == 0 {
		pdf.CellFormat(0, 8, "No se detectaron alertas basadas en firmas o reglas.", "", 1, "L", false, 0, "")
	} else {
		pdf.SetFont("Arial", "B", 8)
		pdf.SetFillColor(darkGray[0], darkGray[1], darkGray[2])
		pdf.SetTextColor(255, 255, 255)
		pdf.CellFormat(30, 7, "Hora", "1", 0, "C", true, 0, "")
		pdf.CellFormat(20, 7, "Severidad", "1", 0, "C", true, 0, "")
		pdf.CellFormat(20, 7, "Origen", "1", 0, "C", true, 0, "")
		pdf.CellFormat(110, 7, "Descripcion / Titulo", "1", 1, "L", true, 0, "")
		
		pdf.SetFont("Arial", "", 8)
		pdf.SetTextColor(0, 0, 0)
		for i, a := range suricataAlerts {
			fill := i%2 == 0
			pdf.SetFillColor(lightGray[0], lightGray[1], lightGray[2])
			pdf.CellFormat(30, 7, a.Time.Format("15:04:05"), "1", 0, "C", fill, 0, "")
			pdf.CellFormat(20, 7, a.Severity, "1", 0, "C", fill, 0, "")
			pdf.CellFormat(20, 7, a.Source, "1", 0, "C", fill, 0, "")
			title := a.Title
			if len(title) > 60 { title = title[:57] + "..." }
			pdf.CellFormat(110, 7, title, "1", 1, "L", fill, 0, "")
		}
	}
	pdf.Ln(5)

	// 7. Resultados del modelo de IA (Anomalias)
	addSectionTitle(pdf, "Resultados del modelo de IA (Machine Learning)")
	if len(mlAlerts) == 0 {
		pdf.CellFormat(0, 8, "No se detectaron anomalias mediante IA en este periodo.", "", 1, "L", false, 0, "")
	} else {
		for _, a := range mlAlerts {
			pdf.SetFont("Arial", "B", 9)
			pdf.SetTextColor(primaryColor[0], primaryColor[1], primaryColor[2])
			pdf.CellFormat(0, 6, fmt.Sprintf("Anomalia detectada a las %s (Score: %.2f)", a.Time.Format("15:04:05"), a.Score), "", 1, "L", false, 0, "")
			pdf.SetTextColor(0, 0, 0)
			pdf.SetFont("Arial", "", 9)
			
			desc := a.Title
			if a.Description != nil { desc = *a.Description }
			
			// Try to parse Context JSON for details
			modelName := "Isolation Forest / Random Forest"
			threshold := "Desconocido"
			if a.Context != nil {
				var ctxData map[string]interface{}
				if err := json.Unmarshal(a.Context, &ctxData); err == nil {
					if m, ok := ctxData["model"].(string); ok { modelName = m }
					if t, ok := ctxData["threshold"].(float64); ok { threshold = fmt.Sprintf("%.2f", t) }
				}
			}

			pdf.MultiCell(0, 5, fmt.Sprintf("Modelo: %s\nUmbral: %s\nJustificacion: %s", modelName, threshold, desc), "", "L", false)
			pdf.Ln(3)
		}
	}
	pdf.Ln(2)

	// 8. Evaluacion de la deteccion (TP/FP/TN/FN)
	if testContext != nil && testContext.ExpectedResult != "" {
		addSectionTitle(pdf, "Evaluacion de la Deteccion (Matriz Avanzada)")
		expected := strings.ToLower(testContext.ExpectedResult)
		isAttackExpected := strings.Contains(expected, "ataque") || strings.Contains(expected, "anomalia") || strings.Contains(expected, "scan") || strings.Contains(expected, "malicioso")
		
		victimIP := strings.ToLower(strings.TrimSpace(testContext.VictimIP))
		attackerIP := strings.ToLower(strings.TrimSpace(testContext.AttackerIP))
		scenarioWords := strings.Fields(strings.ToLower(testContext.ScenarioName))
		
		var relatedAlerts []storage.Alert
		var unrelatedAlerts []storage.Alert
		
		for _, a := range alerts {
			isRelated := false
			desc := strings.ToLower(a.Title)
			if a.Description != nil {
				desc += " " + strings.ToLower(*a.Description)
			}
			
			if victimIP != "" && strings.Contains(desc, victimIP) {
				isRelated = true
			}
			if attackerIP != "" && strings.Contains(desc, attackerIP) {
				isRelated = true
			}
			
			if !isRelated && len(scenarioWords) > 0 {
				matches := 0
				for _, w := range scenarioWords {
					if len(w) > 3 && strings.Contains(desc, w) {
						matches++
					}
				}
				if matches > 0 {
					isRelated = true
				}
			}
			
			if isRelated {
				relatedAlerts = append(relatedAlerts, a)
			} else {
				unrelatedAlerts = append(unrelatedAlerts, a)
			}
		}
		
		binaryResult := "Desconocido"
		classification := "Desconocido"
		
		if isAttackExpected {
			if len(relatedAlerts) > 0 {
				binaryResult = "TRUE POSITIVE (TP)"
				classification = "Correcta (Ataque detectado y correlacionado)"
			} else {
				binaryResult = "FALSE NEGATIVE (FN)"
				classification = "No detectada (El ataque paso inadvertido)"
				if len(unrelatedAlerts) > 0 {
					classification += " [Otras alertas no correlacionadas presentes]"
				}
			}
		} else {
			if len(relatedAlerts) > 0 {
				binaryResult = "FALSE POSITIVE (FP)"
				classification = "Incorrecta (Falsa alarma sobre trafico normal)"
			} else {
				binaryResult = "TRUE NEGATIVE (TN)"
				classification = "Correcta (Trafico normal validado sin alarmas)"
			}
		}
		
		addTableRow(pdf, "Deteccion Binaria:", binaryResult, true)
		addTableRow(pdf, "Clasificacion:", classification, false)
		addTableRow(pdf, "Alertas Relacionadas:", fmt.Sprintf("%d", len(relatedAlerts)), true)
		addTableRow(pdf, "Alertas NO Relacionadas:", fmt.Sprintf("%d", len(unrelatedAlerts)), false)

		pdf.Ln(5)
	}

	// 9. Vulnerabilidades detectadas (Nmap)
	hasRealCVE := false
	for _, v := range vulns {
		if v.CVEID != nil && strings.Contains(strings.ToLower(*v.CVEID), "cve") {
			hasRealCVE = true
			break
		}
	}
	
	nmapTitle := "Servicios expuestos detectados por Nmap"
	if hasRealCVE {
		nmapTitle = "Vulnerabilidades detectadas por Nmap"
	}
	addSectionTitle(pdf, nmapTitle)
	
	if len(vulns) == 0 {
		pdf.CellFormat(0, 8, "No se registraron vulnerabilidades sin mitigar.", "", 1, "L", false, 0, "")
	} else {
		pdf.SetFont("Arial", "B", 8)
		pdf.SetFillColor(darkGray[0], darkGray[1], darkGray[2])
		pdf.SetTextColor(255, 255, 255)
		pdf.CellFormat(30, 7, "Host", "1", 0, "C", true, 0, "")
		pdf.CellFormat(20, 7, "Puerto", "1", 0, "C", true, 0, "")
		pdf.CellFormat(30, 7, "CVE / CVSS", "1", 0, "C", true, 0, "")
		pdf.CellFormat(100, 7, "Descripcion", "1", 1, "L", true, 0, "")
		
		pdf.SetFont("Arial", "", 8)
		pdf.SetTextColor(0, 0, 0)
		for i, v := range vulns {
			fill := i%2 == 0
			pdf.SetFillColor(lightGray[0], lightGray[1], lightGray[2])
			pdf.CellFormat(30, 7, v.DeviceIP, "1", 0, "C", fill, 0, "")
			
			portStr := "-"
			if v.Port != nil { portStr = fmt.Sprintf("%d", *v.Port) }
			pdf.CellFormat(20, 7, portStr, "1", 0, "C", fill, 0, "")
			
			cveStr := "-"
			if v.CVEID != nil { cveStr = *v.CVEID }
			if v.CVSSScore != nil { cveStr += fmt.Sprintf(" (%.1f)", *v.CVSSScore) }
			pdf.CellFormat(30, 7, cveStr, "1", 0, "C", fill, 0, "")
			
			desc := v.Description
			if len(desc) > 60 { desc = desc[:57] + "..." }
			pdf.CellFormat(100, 7, desc, "1", 1, "L", fill, 0, "")
		}
	}
	pdf.Ln(5)

	// 10-12. Analisis y Conclusiones
	addSectionTitle(pdf, "Analisis, Conclusiones y Recomendaciones")
	pdf.SetFont("Arial", "", 10)
	pdf.MultiCell(0, 6, "El analisis del trafico muestra la captura de metadatos de red procesados por los motores de ViewRed.", "", "L", false)
	if len(alerts) > 0 {
		pdf.MultiCell(0, 6, "Se han detectado anomalias o firmas conocidas. Se recomienda aislar los dispositivos implicados (revisar tabla de alertas) y analizar los binarios o procesos involucrados.", "", "L", false)
	} else {
		pdf.MultiCell(0, 6, "No se han detectado anomalias severas en este periodo. Se recomienda continuar con el monitoreo habitual.", "", "L", false)
	}
	if len(vulns) > 0 {
		pdf.MultiCell(0, 6, "Existen vulnerabilidades reportadas por Nmap pendientes de mitigacion. Aplique parches de seguridad lo antes posible.", "", "L", false)
	}
	
	pdf.Ln(5)
	
	// 13. Anexos
	addSectionTitle(pdf, "Anexos tecnicos")
	pdf.SetFont("Arial", "", 9)
	pdf.MultiCell(0, 5, "Este documento fue autogenerado por ViewRed (NDR) a partir de paquetes capturados en tiempo real. La clasificacion mediante IA esta sujeta a falsos positivos dependiendo del entrenamiento del modelo local.", "", "L", false)

	return pdf.OutputFileAndClose(filePath)
}

func addSectionTitle(pdf *fpdf.Fpdf, title string) {
	pdf.SetFont("Arial", "B", 12)
	pdf.SetTextColor(211, 47, 47)
	pdf.CellFormat(190, 8, title, "B", 1, "L", false, 0, "")
	pdf.Ln(3)
	pdf.SetTextColor(0, 0, 0)
}

func addTableRow(pdf *fpdf.Fpdf, label string, value string, fill bool) {
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(60, 7, label, "1", 0, "L", fill, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(130, 7, value, "1", 1, "R", fill, 0, "")
}

func formatBytes(b float64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%.2f B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatBytesFloat(b float64) string {
	return formatBytes(b)
}
