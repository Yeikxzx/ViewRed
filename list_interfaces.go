package main
import (
	"fmt"
	"log"
	"github.com/google/gopacket/pcap"
)
func main() {
	devices, err := pcap.FindAllDevs()
	if err != nil {
		log.Fatal(err)
	}
	for _, dev := range devices {
		fmt.Printf("Name: %s\nDescription: %s\n\n", dev.Name, dev.Description)
	}
}
