package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
)

func Check(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {
	arguments := os.Args
	if len(arguments) < 3 {
		fmt.Println("Please provide a valid port!")
		return
	}

	if arguments[1] == "client" {
		UDPClient()
		os.Exit(0)
	}

	if arguments[1] != "server" {
		fmt.Println("Please provide a mode: client or server !")
		os.Exit(1)
	}

	ADDR := ":" + arguments[2]

	s, err := net.ResolveUDPAddr("udp4", ADDR)
	if err != nil {
		fmt.Println(err)
		return
	}

	connection, err := net.ListenUDP("udp4", s)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer connection.Close()

	fbuffer := make([]byte, 32000)
	buffer := make([]byte, 32000)
	_, err = os.ReadFile("./msg.txt")
	Check(err)

	f, err := os.Open("./msg.txt")
	Check(err)

	_, err = f.Read(fbuffer)
	Check(err)

	count := 0
	for {
		len, addr, err := connection.ReadFromUDP(buffer)
		abuf := make([]byte, len)
		copy(abuf, buffer)
		//fmt.Print("-> ", string(buffer[:]))
		count++
		if count == 1 && !bytes.Equal(abuf, fbuffer) {
			fmt.Println("unexpected msg %v", abuf)
			continue
		}

		_, err = connection.WriteToUDP(abuf, addr)
		if err != nil {
			fmt.Println(err)
			continue
		}

	}
}

func UDPClient() {

	if len(os.Args) < 2 {
		fmt.Println("Please provide host:port to connect to")
		os.Exit(1)
	}

	// Resolve the string address to a UDP address
	udpAddr, err := net.ResolveUDPAddr("udp", os.Args[2])

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fbuffer := make([]byte, 32000)
	_, err = os.ReadFile("./msg.txt")
	Check(err)

	f, err := os.Open("./msg.txt")
	Check(err)

	_, err = f.Read(fbuffer)
	Check(err)

	_, err = conn.Write(fbuffer)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	buffer := make([]byte, 32000)
	_, _, err = conn.ReadFromUDP(buffer)
	if err != nil {
		fmt.Println(err)
		return
	}

	if !bytes.Equal(buffer, fbuffer) {
		fmt.Printf("FAIL1 : Len %d\n", len(buffer))
		os.Exit(1)
	}

	// Send another message to the server
	f, err = os.Open("./msg_short.txt")
	Check(err)

	_, err = f.Read(fbuffer)
	Check(err)

	_, err = conn.Write(fbuffer)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Read from the connection untill a new line is send
	_, _, err = conn.ReadFromUDP(buffer)
	if err != nil {
		fmt.Println(err)
		return
	}

	if bytes.Equal(buffer, fbuffer) {
		fmt.Println("OK")
	} else {
		fmt.Printf("FAIL2 : Len %s\n", len(buffer))
		os.Exit(1)
	}
}
