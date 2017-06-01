package main

import "os"
import "net"
import "plus"
import "plus/packet"
import "fmt"

func showUsage() {
}

func main() {

	args := os.Args

	if len(args) < 2 {
		showUsage()
		return
	}

	mode := args[1]

	var laddr string
	var raddr string
	var lfaddr string

	switch mode {
		case "drop":
			if len(args) < 3 {
				showUsage()
				return
			}

			laddr = args[2]
			raddr = ""
			lfaddr = ""

		case "echo":
			if len(args) < 3 {
				showUsage()
				return
			}

			laddr = args[2]
			raddr = ""
			lfaddr = ""

		case "forward":
			if len(args) < 5 {
				showUsage()
				return
			}

			laddr = args[2]
			raddr = args[3]
			lfaddr = args[4]

		default:
			panic("Invalid mode!")
	}

	packetConn, err := net.ListenPacket("udp", laddr)

	if err != nil {
		panic("Could not create packet connection.")
	}

	udpAddr, err := net.ResolveUDPAddr("udp", raddr)

	if err != nil {
		panic("Could not resolve remote address.")
	}

	run(packetConn, udpAddr, lfaddr, mode)
}

func plusPacketToString(plusPacket *packet.PLUSPacket, from net.Addr, to net.Addr, mode string) string {
	if !plusPacket.XFlag() {
			strFmt := "{\"mode\":\"%s\",\"from\":\"%s\",\"to\":\"%s\",\"flags\":{\"l\":%t,\"r\":%t,\"s\":%t,\"x\":%t},\"psn\":%d,\"pse\":%d,\"cat\":%d}"
			strOut := fmt.Sprintf(strFmt, mode, from.String(), to.String(), plusPacket.LFlag(), 
						plusPacket.RFlag(), plusPacket.SFlag(),
						plusPacket.XFlag(), plusPacket.PSN(),
						plusPacket.PSE(), plusPacket.CAT())

			return strOut
	} else {
		strFmt := "{\"mode\":\"%s\",\"from\":\"%s\",\"to\":\"%s\",\"flags\":{\"l\":%t,\"r\":%t,\"s\":%t,\"x\":%t},\"psn\":%d,\"pse\":%d,\"cat\":%d,\"pcflen\":%d,\"pcfintegrity\":%d,\"pcftype\":%d,\"pcfvalue\":%d}"

		var pcfType_ int
		var pcfLen_ int
		var pcfIntegrity_ int

		pcfType, err := plusPacket.PCFType()

		if err != nil {
			pcfType_ = -1
		} else {
			pcfType_ = int(pcfType)
		}

		pcfIntegrity, err := plusPacket.PCFIntegrity()

		if err != nil {
			pcfIntegrity_ = -1
		} else {
			pcfIntegrity_ = int(pcfIntegrity)
		}

		pcfLen, err := plusPacket.PCFLen()

		if err != nil {
			pcfLen_ = -1
		} else {
			pcfLen_ = int(pcfLen)
		}

		pcfValue, err := plusPacket.PCFValue()

		if err != nil {
			pcfValue = []byte{}
		}

		strOut := fmt.Sprintf(strFmt, mode, from.String(), to.String(), plusPacket.LFlag(), 
					plusPacket.RFlag(), plusPacket.SFlag(),
					plusPacket.XFlag(), plusPacket.PSN(),
					plusPacket.PSE(), plusPacket.CAT(),
					pcfLen_, pcfIntegrity_, pcfType_, pcfValue)

		return strOut
	}
}

func run(packetConn net.PacketConn, remoteAddr *net.UDPAddr, lfaddr string, mode string) {
	drop := false


	if mode == "drop" {
		drop = true
	}

	connManager := PLUS.NewConnectionManager(packetConn)

	var forwardConn net.PacketConn
	var forwardConnManager *PLUS.ConnectionManager

	if mode == "forward" {
		var err error
		forwardConn, err = net.ListenPacket("udp", lfaddr)

		if err != nil {
			panic("Could not create packet connection.")
		}

		forwardConnManager = PLUS.NewConnectionManager(forwardConn)

		go func() {
			for {
				plusPacket, addr, err := forwardConnManager.ReadPacket()

				if err != nil {
					panic("oops!")
				}

				fmt.Println(plusPacketToString(plusPacket, addr, forwardConn.LocalAddr(), "forward:in"))

				connection, err := connManager.GetConnection(plusPacket.CAT())

				if err != nil {
					panic("oops!")
				}

				fmt.Println(plusPacketToString(plusPacket, forwardConn.LocalAddr(), connection.RemoteAddr(), "forward:back"))
				forwardConnManager.WritePacket(plusPacket, connection.RemoteAddr())

				
			}
		}()
	}

	for {
		connection, plusPacket, addr, _, err := connManager.ReadAndProcessPacket()

		if err != nil {
			panic("oops!")
		}

		fmt.Println(plusPacketToString(plusPacket, addr, packetConn.LocalAddr(), "in"))

		if !drop {
			switch mode {
			case "echo":
				echoPacket, _ := connection.PrepareNextPacket()
				echoPacket.SetPayload(plusPacket.Payload())
				
				fmt.Println(plusPacketToString(echoPacket, packetConn.LocalAddr(), addr, "echo:out"))
				connManager.WritePacket(echoPacket, addr)

			case "forward":
				fmt.Println(plusPacketToString(plusPacket, forwardConn.LocalAddr(), remoteAddr, "forward:out"))
				forwardConnManager.WritePacket(plusPacket, remoteAddr)
			}
		}
	}

}
