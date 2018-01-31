package main

import "os"
import "net"
import "github.com/mami-project/plus-lib"
import "github.com/mami-project/plus-lib/packet"
import "fmt"
import "strconv"
import "time"
import "math/rand"

var rndCat = uint64(rand.Uint32())<<32 | uint64(rand.Uint32())

func showUsage() {
	fmt.Println(".[SYNTAX].")
	fmt.Println("")
	fmt.Println("plusspector <mode> <arg1> <arg2> ...")
	fmt.Println("")
	fmt.Println("pluspector drop       localAddr")
	fmt.Println("pluspector echo       localAddr")
	fmt.Println("pluspector forward    localAddr remoteAddr localRelayAddr")
	fmt.Println("pluspector client     localAddr remoteAddr")
	fmt.Println("pluspector drop-rate  localAddr remoteAddr numPackets packetSize sendDelay")
	fmt.Println("pluspector fuzz       localAddr remoteAddr f")
	fmt.Println("")
	fmt.Println(" localAddr:      local address connection will listen on")
	fmt.Println(" remoteAddr:     destination address to connect to")
	fmt.Println(" localRelayAddr: when in forward mode packets will be forwarded")
	fmt.Println("                 from this local address")
	fmt.Println(" numPackets:     how many packets to send")
	fmt.Println(" packetSize:     size of a packet")
	fmt.Println(" sendDelay:      time to wait between two packets (milliseconds)")
	fmt.Println(" f:              how hard to fuzz (on a scale from 1 to 100)")
	fmt.Println("")
	fmt.Println(".[MODES].")
	fmt.Println("")
	fmt.Println("drop")
    fmt.Println(" Listen for packets and drop them.")
	fmt.Println("")
	fmt.Println("echo")
    fmt.Println(" Listen for packets and echo them back.")
	fmt.Println(" The first byte of an echo packet indicates whether")
	fmt.Println(" the echo packet contains the echoed payload or")
	fmt.Println(" whether it contains feedback.")
	fmt.Println("  - 0x00 = regular data.")
	fmt.Println("  - 0xFF = feedback data.")
	fmt.Println("")
	fmt.Println("client")
	fmt.Println(" Send 4k packets with every fourth packet")
	fmt.Println(" containing a PCF request (0x00, 0x00, [00])")
    fmt.Println(" packets are sent as fast as possible")
	fmt.Println("")
	fmt.Println("forward")
	fmt.Println(" Listen for packets and forward them on a different")
    fmt.Println(" local relay address to the remote address while listening")
	fmt.Println("  for packets from the remote address on the local relay adress")
	fmt.Println("  and send those packets back to the original sender.")
	fmt.Println("")
	fmt.Println("drop-rate")
	fmt.Println(" Measure ratio of sent packets to received packets. ")
	fmt.Println(" Also measures difference of sent PSN and received PSE.")
	fmt.Println("")
	fmt.Println("fuzz")
	fmt.Println(" Like client mode but randomly manipulate packets to see")
	fmt.Println(" how the receiver deals with it.")
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

	case "client":
		if len(args) < 4 {
			showUsage()
			return
		}

		laddr = args[2]
		raddr = args[3]

		client(laddr, raddr)
		return

	case "fuzz":
		if len(args) < 5 {
			showUsage()
			return
		}

		laddr = args[2]
		raddr = args[3]
		f, err := strconv.ParseInt(args[4], 10, 16)

		if err != nil {
			panic("Not a number!")
		}

		fuzz(laddr, raddr, int(f))
		return

	case "drop-rate":
		if len(args) < 7 {
			showUsage()
			return
		}

		laddr = args[2]
		raddr = args[3]
		numPackets, err := strconv.ParseInt(args[4], 10, 16)

		if err != nil {
			panic("Not a number!")
		}

		packetSize, err := strconv.ParseInt(args[5], 10, 16)

		if err != nil {
			panic("Not a number!")
		}

		sendDelay, err := strconv.ParseInt(args[6], 10, 16)

		if err != nil {
			panic("Not a number!")
		}

		dropRate(laddr, raddr, numPackets, packetSize, sendDelay)
		return
		

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

func shuffle(data []byte) {
   sz := len(data)
	for i, _ := range data {
		j := rand.Int() % sz
		t := data[i]
		data[i] = data[j]
		data[j] = t
	}
}

func genData(size int64) []byte {
	data := make([]byte, int(size))

	for i:= int64(0); i < size; i++ {
		data[i] = byte(rand.Int() % 256)
	}

	return data
}

func mutilate(plusPacket *packet.PLUSPacket, f int) []byte {
	buffer := plusPacket.Buffer()

	for i := 0; i < f; i++ {
		idx := rand.Int() % len(buffer)

		mask := byte(rand.Int() % 256)

		buffer[idx] ^= mask
	}

	return buffer
}

func fuzz(laddr string, remoteAddr string, f int) {
	packetConn, err := net.ListenPacket("udp", laddr)

	if err != nil {
		panic("Could not create packet connection!")
	}

	udpAddr, err := net.ResolveUDPAddr("udp4", remoteAddr)

	if err != nil {
		panic("Could not resolve address!")
	}

	connectionManager, connection := PLUS.NewConnectionManagerClient(packetConn, rndCat, udpAddr)



	go func() {
		for {
			timeout := make(chan bool)
			got := make(chan bool)

			go func() {
				time.Sleep(1000 * time.Millisecond)
				timeout <- true
			}()

			go func() {
				_, _, _, _, err := connectionManager.ReadAndProcessPacket()
				if err != nil {
					fmt.Println(err.Error())
					// ignore errors here
				}
				got <- true
			}()

			select {
				case <- timeout:
					panic("Receiver crashed \\o/")
				case <- got:
					// meh
			}
		}
	}()



	for i := int64(0); i < int64(4294967296); i++ {
		buffer := genData(int64(rand.Int() % 4096))

		if i % 4 == 0 {
			connection.QueuePCFRequest(0x01, 0, []byte{0x00})
		}

		plusPacket, err := connection.PrepareNextPacket()
		plusPacket.SetPayload(buffer)

		if err != nil {
			panic("oops!")
		}

		_, err = packetConn.WriteTo(mutilate(plusPacket, f), udpAddr)

		if err != nil {
			panic("oops!")
		}
	}
}

func dropRate(laddr string, remoteAddr string, numPackets int64, packetSize int64, sendDelay int64) {
	packetConn, err := net.ListenPacket("udp", laddr)

	if err != nil {
		panic("Could not create packet connection!")
	}

	udpAddr, err := net.ResolveUDPAddr("udp4", remoteAddr)

	if err != nil {
		panic("Could not resolve address!")
	}

	connectionManager, connection := PLUS.NewConnectionManagerClient(packetConn, rndCat, udpAddr)

	received := int64(0)
	max := int64(0)
	min := int64(4294967296)
	sum := int64(0)

	go func() {
		for {
			connection, inPacket, addr, _, err := connectionManager.ReadAndProcessPacket()
			if err != nil {
				panic("oops!")
			}

			psn := connection.PSN()
			pse := inPacket.PSE()

			diff := int64(psn) - int64(pse)

			if diff > max {
				max = diff
			}

			if diff < min {
				min = diff
			}

			sum += diff

			received++

			fmt.Printf("--")
			fmt.Println(plusPacketToString(inPacket, addr, packetConn.LocalAddr(), "client:in"))
		}
	}()

	buffer := genData(packetSize)

	for i := int64(0); i < numPackets; i++ {
		shuffle(buffer)

		if i % 4 == 0 {
			connection.QueuePCFRequest(0x01, 0, []byte{0x00})
		}

		plusPacket, err := connection.PrepareNextPacket()
		plusPacket.SetPayload(buffer)

		if err != nil {
			panic("oops!")
		}

		err = connectionManager.WritePacket(plusPacket, udpAddr)

		if err != nil {
			panic("oops!")
		}

		fmt.Printf("--")
		fmt.Println(plusPacketToString(plusPacket, packetConn.LocalAddr(), udpAddr, "client:out"))

		time.Sleep(time.Duration(sendDelay) * time.Millisecond)
	}

	time.Sleep(time.Duration(1000) * time.Millisecond)

	fmt.Printf("{\"sent\":%d,\"received\":%d,\"max\":%d,\"min\":%d,\"avg\":%d}\n", numPackets, received, max, min, sum / received)
}

func client(laddr string, remoteAddr string) {
	packetConn, err := net.ListenPacket("udp", laddr)

	if err != nil {
		panic("Could not create packet connection!")
	}

	udpAddr, err := net.ResolveUDPAddr("udp4", remoteAddr)

	if err != nil {
		panic("Could not resolve address!")
	}

	connectionManager, connection := PLUS.NewConnectionManagerClient(packetConn, rndCat, udpAddr)

	go func() {
		for {
			_, inPacket, addr, _, err := connectionManager.ReadAndProcessPacket()
			if err != nil {
				panic("oops!")
			}

			fmt.Printf(plusPacketToString(inPacket, addr, packetConn.LocalAddr(), "client:in"))
		}
	}()

	for i := 0; i < 4096; i++ {
		if i % 4 == 0 {
			connection.QueuePCFRequest(0x01, 0, []byte{0x00})
		}

		buffer := []byte{0x00, 0x65, 0x66, 0x67, 0x68}

		plusPacket, err := connection.PrepareNextPacket()
		plusPacket.SetPayload(buffer)

		if err != nil {
			panic("oops!")
		}

		err = connectionManager.WritePacket(plusPacket, udpAddr)

		if err != nil {
			panic("oops!")
		}

		fmt.Println(plusPacketToString(plusPacket, packetConn.LocalAddr(), udpAddr, "client:out"))

		time.Sleep(time.Duration(1) * time.Millisecond)
	}
}

func plusPacketToString(plusPacket *packet.PLUSPacket, from net.Addr, to net.Addr, mode string) string {
	if plusPacket == nil {
		return "{\"error\":\"n/a\"}"
	}

	if !plusPacket.XFlag() {
		strFmt := "{\"mode\":\"%s\",\"from\":\"%s\",\"to\":\"%s\",\"flags\":{\"l\":%t,\"r\":%t,\"s\":%t,\"x\":%t},\"psn\":%d,\"pse\":%d,\"cat\":%d,\"payload\":%d}"
		strOut := fmt.Sprintf(strFmt, mode, from.String(), to.String(), plusPacket.LFlag(),
			plusPacket.RFlag(), plusPacket.SFlag(),
			plusPacket.XFlag(), plusPacket.PSN(),
			plusPacket.PSE(), plusPacket.CAT(), plusPacket.Payload())

		return strOut
	} else {
		strFmt := "{\"mode\":\"%s\",\"from\":\"%s\",\"to\":\"%s\",\"flags\":{\"l\":%t,\"r\":%t,\"s\":%t,\"x\":%t},\"psn\":%d,\"pse\":%d,\"cat\":%d,\"pcflen\":%d,\"pcfintegrity\":%d,\"pcftype\":%d,\"pcfvalue\":%d,\"payload\":%d}"

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
			pcfLen_, pcfIntegrity_, pcfType_, pcfValue, plusPacket.Payload())

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
					fmt.Printf("-- ERROR: %s\n", err.Error())
					// ignore this
					continue
				}

				fmt.Println(plusPacketToString(plusPacket, forwardConn.LocalAddr(), connection.RemoteAddr(), "forward:back"))
				forwardConnManager.WritePacket(plusPacket, connection.RemoteAddr())

			}
		}()
	}

	for {
		// plusPacket, addr, err := connManager.ReadPacket()

		connection, plusPacket, addr, feedback, err := connManager.ReadAndProcessPacket()

		if err != nil {
			fmt.Printf("-- ERROR: %s\n", err.Error())
			// ignore this
			continue
		}

		fmt.Println(plusPacketToString(plusPacket, addr, packetConn.LocalAddr(), "in"))

		if !drop {
			switch mode {
			case "echo":
				echoPacket, _ := connection.PrepareNextPacket()

				var payload []byte

				if feedback == nil {
					payload = make([]byte, len(plusPacket.Payload())+1)
					payload[0] = 0x00
					copy(payload[1:], plusPacket.Payload())
				} else {
					payload = make([]byte, len(feedback)+1)
					payload[0] = 0xFF
					copy(payload[1:], feedback)
				}

				echoPacket.SetPayload(payload)

				fmt.Println(plusPacketToString(echoPacket, packetConn.LocalAddr(), addr, "echo:out"))
				connManager.WritePacket(echoPacket, addr)

			case "forward":
				fmt.Println(plusPacketToString(plusPacket, forwardConn.LocalAddr(), remoteAddr, "forward:out"))
				forwardConnManager.WritePacket(plusPacket, remoteAddr)
			}
		}
	}

}
