// fixclient project main.go
package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const SOH = string(1)
const CTOUT = 3

var fixVer = flag.String("v", "4.3", "FIX protocol version")
var target = flag.String("h", "", "Target host")
var targetPort = flag.String("p", "", "Target port")
var SenderCompID = flag.String("s", "MySender", "SenderCompID")
var TargetCompID = flag.String("t", "MyTarget", "TargetCompID")
var inputFile = flag.String("f", "input.log", "Input file")
var HeartBeat = flag.Duration("b", 30*time.Second, "HeartBeat")
var useTls = flag.Bool("x", false, "Use TLS")
var seq int
var BeginString string

func ckErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: ", err)
		os.Exit(1)
	}
}

func TimeStamp() string {
	t := time.Now().UTC()
	return fmt.Sprintf("%d%02d%02d-%02d:%02d:%02d.%03d",
		t.Year(), int(t.Month()), t.Day(), t.Hour(),
		t.Minute(), t.Second(), t.Nanosecond()/1000000)
}

func Log(s string, direction string) {
	_, err := fmt.Printf("%s %s %s\n", TimeStamp(), direction, s)
	ckErr(err)
}

func Checksum(msg []byte) uint {
	var cksum uint
	for _, val := range []byte(msg) {
		cksum = cksum + uint(val)
	}
	return uint(math.Mod(float64(cksum), 256))
}

func Heartbeat(hb time.Duration) <-chan string {
	c := make(chan string)
	go func() {
		for i := 0; ; i++ {
			c <- fmt.Sprintf("8=%s|35=0|49=|56=|34=|52=|", BeginString)
			time.Sleep(hb)
		}
	}()
	return c
}

func Remote(conn net.Conn) <-chan []byte {
	c := make(chan []byte)
	go func() {
		scanner := bufio.NewScanner(conn)
		// debug
		//scanner.Split(ScanFIX)
		for scanner.Scan() {
			c <- scanner.Bytes()
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "reading from remote:", err)
			os.Exit(1)
		}
	}()
	return c
}

func Parse(s string) (parsed string) {
	if strings.HasPrefix(s, "8=FIX") {
		input := strings.Split(s, "|")
		var body string
		// let's construct message body from input
		for _, v := range input {
			m := strings.Split(v, "=")
			if m[0] == "" ||
				m[0] == "8" ||
				m[0] == "9" ||
				m[0] == "10" {
				continue
			} else {
				tag, err := strconv.Atoi(m[0])
				ckErr(err)
				switch {
				case tag == 34:
					body = fmt.Sprintf("%s%d=%d|", body, tag, seq)
				case tag == 49:
					body = fmt.Sprintf("%s%d=%s|", body, tag, *SenderCompID)
				case tag == 52:
					body = fmt.Sprintf("%s%d=%s|", body, tag, TimeStamp())
				case tag == 56:
					body = fmt.Sprintf("%s%d=%s|", body, tag, *TargetCompID)
				default:
					body = fmt.Sprintf("%s%d=%s|", body, tag, m[1])
				}
			}
		}
		header := fmt.Sprintf("8=%s|9=%d|", BeginString, len(body))
		message := header + body
		parsed = strings.Replace(message, "|", SOH, -1)
		cksum := Checksum([]byte(parsed))
		parsed = fmt.Sprintf("%s10=%03d%s", parsed, cksum, SOH)
	}
	return
}

func Scenario(f *os.File, intercom chan string) <-chan string {
	c := make(chan string)
	go func() {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case strings.HasPrefix(line, "8=FIX"):
				if strings.Contains(line, "$RANDOM") {
					rand.Seed(time.Now().UnixNano())
					r := rand.Intn(1000)
					c <- strings.Replace(line, "$RANDOM", strconv.Itoa(r), -1)
				} else {
					c <- line
				}
			case strings.HasPrefix(line, "sleep "):
				sleep, err := time.ParseDuration(strings.Fields(line)[1])
				ckErr(err)
				Log("sleeping for "+sleep.String(), "-")
				time.Sleep(sleep)
			case strings.HasPrefix(line, "exit"):
				Log("exit", "-")
				os.Exit(0)
			case strings.HasPrefix(line, "expect "):
				expect := strings.Fields(line)[1]
				Log("expecting "+expect, "-")
				intercom <- "1"
				msg := <-intercom
				if strings.Contains(msg, SOH+expect+SOH) != true {
					Log("expected "+expect+" but got unexpected "+msg, "-")
					os.Exit(1)
				}
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "reading from file:", err)
			os.Exit(1)
		}
	}()
	return c
}

func Send(conn net.Conn, s string) {
	parsed := Parse(s)
	message := strings.Replace(parsed, SOH, "|", -1)
	Log(message, "<")
	_, err := fmt.Fprintf(conn, "%s", parsed)
	ckErr(err)
	seq = seq + 1
}

// ScanFIX is a split function for a Scanner that returns each
// FIX messsage.
func ScanFIX(data []byte, isEOF bool) (advance int, token []byte, err error) {
	if isEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.Index(data, []byte(SOH+"10=")); i >= 0 {
		// Check if we have tag 10 followed by SOH.
		if len(data)-i >= 8 {
			return i + 8, data[0 : i+8], nil
		}
	}
	// If EOF, we have a final, non-SOH terminated message.
	if isEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}

func main() {
	flag.Parse()
	if *target == "" || *targetPort == "" {
		flag.Usage()
		os.Exit(2)
	}

	f, err := os.Open(*inputFile)
	ckErr(err)

	var conn net.Conn
	if *useTls {
		tlsConf := new(tls.Config)
		tlsConf.InsecureSkipVerify = true
		conn, err = tls.Dial("tcp", *target+":"+*targetPort, tlsConf)
	} else {
		conn, err = net.DialTimeout("tcp", *target+":"+*targetPort,
			CTOUT*time.Second)
	}
	ckErr(err)
	defer conn.Close()

	//initialize sequence
	seq = 1
	BeginString = fmt.Sprintf("FIX.%s", *fixVer)

	intercom := make(chan string)
	scenario := Scenario(f, intercom)
	remote := Remote(conn)
	hearbeat := Heartbeat(*HeartBeat)

	for {
		select {
		case v1 := <-hearbeat:
			Send(conn, v1)
		case v2 := <-remote:
			Log(strings.Replace(string(v2), SOH, "|", -1), ">")
			//check if there is expect command waiting for input
			select {
			case <-intercom:
				intercom <- strings.Replace(string(v2), SOH, "|", -1)
			default:
			}
		case v3 := <-scenario:
			Send(conn, v3)
		}
	}
}
