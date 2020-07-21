package tcp

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

/*
Packet represent data and layers of packet.
parser extracts information from pcap Packet. functions of *Packet doesn't validate if packet is nil,
calllers must make sure that ParsePacket has'nt returned any error before calling any other
function.
*/
type Packet struct {
	// Link layer
	gopacket.LinkLayer

	// IP Header
	gopacket.NetworkLayer

	// TCP Segment Header
	*layers.TCP

	// Application Layer(data layer)
	DataLayer gopacket.ApplicationLayer

	// Data info
	Lost      uint16
	Timestamp time.Time
}

// ParsePacket parse raw packets
func ParsePacket(packet gopacket.Packet) (pckt *Packet, err error) {
	defer func() {
		e := packet.ErrorLayer()
		if e != nil {
			if _, ok := e.(*gopacket.DecodeFailure); ok {
				err = e.(*gopacket.DecodeFailure).Error()
				return
			}
			err = e.Error()
		}
	}()

	// initialization
	pckt = new(Packet)
	pckt.Timestamp = packet.Metadata().Timestamp
	if pckt.Timestamp.Equal(time.Time{}) {
		pckt.Timestamp = time.Now()
	}

	// parsing link layer
	pckt.LinkLayer = packet.LinkLayer()

	// parsing network layer
	if net4, ok := packet.NetworkLayer().(*layers.IPv4); ok {
		pckt.NetworkLayer = net4
	} else if net6, ok := packet.NetworkLayer().(*layers.IPv6); ok {
		pckt.NetworkLayer = net6
	} else {
		return
	}

	// parsing tcp header(transportation layer)
	if tcp, ok := packet.TransportLayer().(*layers.TCP); ok {
		pckt.TCP = tcp
	} else {
		return
	}
	pckt.DataOffset *= 4

	// parsing application later(actual data)
	pckt.DataLayer = packet.ApplicationLayer()

	// calculating lost data
	headerSize := int(uint32(pckt.DataOffset) + uint32(pckt.IHL()))
	if pckt.Version() == 6 {
		headerSize = int(pckt.DataOffset) // in ipv6 the length of payload doesn't include the IPheader size
	}
	pckt.Lost = pckt.Length() - uint16(headerSize+len(pckt.Payload))

	return
}

// Src format the source socket of a packet
func (pckt *Packet) Src() string {
	return fmt.Sprintf("%s:%d", pckt.SrcIP(), pckt.SrcPort)
}

// Dst format destination socket
func (pckt *Packet) Dst() string {
	return fmt.Sprintf("%s:%d", pckt.DestIP(), pckt.DstPort)
}

// SrcIP returns source IP address
func (pckt *Packet) SrcIP() net.IP {
	if pckt.Version() == 4 {
		return pckt.NetworkLayer.(*layers.IPv4).SrcIP
	}
	return pckt.NetworkLayer.(*layers.IPv6).SrcIP
}

// DestIP returns destination IP address
func (pckt *Packet) DestIP() net.IP {
	if pckt.Version() == 4 {
		return pckt.NetworkLayer.(*layers.IPv4).DstIP
	}
	return pckt.NetworkLayer.(*layers.IPv6).DstIP
}

// Version returns version of IP protocol
func (pckt *Packet) Version() uint8 {
	if _, ok := pckt.NetworkLayer.(*layers.IPv4); ok {
		return 4
	}
	return 6
}

// IHL returns IP header length in bytes
func (pckt *Packet) IHL() uint8 {
	if l, ok := pckt.NetworkLayer.(*layers.IPv4); ok {
		return l.IHL * 4
	}
	// on IPV6 it's constant, https://en.wikipedia.org/wiki/IPv6_packet#Fixed_header
	return 40
}

// Length returns the total length of the packet(IP header, TCP header and the actual data)
func (pckt *Packet) Length() uint16 {
	if l, ok := pckt.NetworkLayer.(*layers.IPv4); ok {
		return l.Length
	}
	return pckt.NetworkLayer.(*layers.IPv6).Length
}

// SYNOptions returns MSS and windowscale of syn packets
func (pckt *Packet) SYNOptions() (mss, windowscale uint16) {
	if !pckt.SYN {
		return
	}
	for _, v := range pckt.Options {
		if v.OptionType == layers.TCPOptionKindMSS {
			mss = binary.BigEndian.Uint16(v.OptionData)
			continue
		}
		if v.OptionType == layers.TCPOptionKindWindowScale {
			if v.OptionLength > 0 {
				windowscale = 1 << v.OptionData[0] // 2 ** windowscale
			}
		}
	}
	return
}

// Flag returns formatted tcp flags
func (pckt *Packet) Flag() (flag string) {
	if pckt.FIN {
		flag += "FIN, "
	}
	if pckt.SYN {
		flag += "SYN, "
	}
	if pckt.RST {
		flag += "RST, "
	}
	if pckt.PSH {
		flag += "PSH, "
	}
	if pckt.ACK {
		flag += "ACK, "
	}
	if pckt.URG {
		flag += "URG, "
	}
	if len(flag) != 0 {
		return flag[:len(flag)-2]
	}
	return flag
}

// String output for a TCP Packet
func (pckt *Packet) String() string {
	return fmt.Sprintf(`Time: %s
Source: %s
Destination: %s
IHL: %d
Total Length: %d
Sequence: %d
Acknowledgment: %d
DataOffset: %d
Window: %d
Flag: %s
Options: %s
Data Size: %d
Lost Data: %d`,
		pckt.Timestamp.Format(time.StampNano),
		pckt.Src(),
		pckt.Dst(),
		pckt.IHL(),
		pckt.Length(),
		pckt.Seq,
		pckt.Ack,
		pckt.DataOffset,
		pckt.Window,
		pckt.Flag(),
		pckt.Options,
		len(pckt.Payload),
		pckt.Lost,
	)
}
