package fireclient

import (
	"net"
	"testing"
	"time"
)

func startTestServer() {

}

func TestBroadcast(t *testing.T) {
	server, err := net.ListenUDP("udp", &net.UDPAddr{Port: 8080})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	t.Logf("Local address: %s", server.LocalAddr().String())

	client, err := net.Dial("udp", server.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	ra, err := net.ResolveUDPAddr("udp", server.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}

	// packet,err:= firecontrol.

	b := []byte("CONNECTED-MODE SOCKET")
	_, err = client.(*net.UDPConn).WriteToUDP(b, ra)
	if err == nil {
		t.Fatal("should fail")
	}

	// testWriteToConn(t, c.LocalAddr().String())
}

func TestUDPServer(t *testing.T) {
	go func() {
		// main() // Start the UDP server
	}()
	time.Sleep(1 * time.Second) // Give the server time to start

	serverAddr := net.UDPAddr{
		Port: 8080,
		IP:   net.ParseIP("127.0.0.1"),
	}

	conn, err := net.DialUDP("udp", nil, &serverAddr)
	if err != nil {
		t.Fatalf("Failed to dial UDP server: %v", err)
	}
	defer conn.Close()

	_, err = conn.Write([]byte("Test message"))
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	buffer := make([]byte, 1024)
	n, _, err := conn.ReadFromUDP(buffer)
	if err != nil {
		t.Fatalf("Failed to receive message: %v", err)
	}

	expected := "Message received"
	if string(buffer[:n]) != expected {
		t.Fatalf("Expected %s, got %s", expected, string(buffer[:n]))
	}
}
