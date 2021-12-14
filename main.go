package main

import (
	"bufio"
	"flag"
	"io"
	"log"
	"net"
	"os"
	"strconv"
)

func main() {
	ParseConfig()
	StartServer()
	select {} // block forever, this program exits by ctrl+c :)
}

//stopper exists the main listener loop when set to true
var stopper bool = false

//data-log file name for downlink
var ddf *os.File

//data-log file name for uplink
var duf *os.File

//StartServer starts the forwarding server, which listens for connections and forwards them according to configuration
//configuration is expected to be in the global Config object/struct/whatever
func StartServer() {
	//create a log file and configure logging to both standard output and the file
	if Config.logFile != "" {
		f, err := os.OpenFile(Config.logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("Failed to open log file for writing: %v", err)
		}
		if !Config.logToConsole {
			log.SetOutput(io.MultiWriter(os.Stdout, f))
		} else {
			log.SetOutput(io.MultiWriter(f))
		}
	} else {
		if Config.logToConsole {
			log.SetOutput(io.MultiWriter(os.Stdout))
		}
	}

	listener, err := net.Listen("tcp", "0.0.0.0:"+strconv.Itoa(Config.srcPort))
	if err != nil {
		log.Fatalf("Listen failed: %v", err)
	}
	//close the listener when this function exits
	defer listener.Close()
	debuglog("Listening for connection")

	for {
		if stopper {
			//exit if someone set the flag to stop forwarding
			break
		}
		mainConn, err := listener.Accept()
		debuglog("Got connection %v -> %v", mainConn.RemoteAddr(), mainConn.LocalAddr())
		if err != nil {
			//TODO; should probably not exit on error from one client..
			debuglog("server error")
			listener.Close()
			log.Fatalln(err)
		}
		//start a new thread for this connection and wait for the next one
		go forward(mainConn, Config.target)
	}
}

//writes to log if logging to console or file is enabled
//honestly, I just copied the v... interface{} from the log package definition so there you go
func debuglog(msg string, v ...interface{}) {
	if Config.logToConsole || Config.logFile != "" {
		log.Printf(msg, v...)
	}
}

//forward() handles forwarding of a given source connection to configured destion/mirror
func forward(srcConn net.Conn, target string) {
	//have to defer here as defer waits for surrounding function to return.
	//deferring in main() for loop would only execute when main() exits (?)
	defer srcConn.Close()

	//set up main destination, the one whose returned data is also written back to source connection
	debuglog("Target : %s", target)
	dstConn, err := net.Dial("tcp", target)

	if err != nil {
		//try not to fail on a single error when forwarding a single connection. maybe destination is down and will be up, or maybe there is temporary network outage etc?
		log.Printf("Connection to destination failed. Skipping connection. Error: %v", err)
		return
	}
	debuglog("Dialed %v -> %v", dstConn.LocalAddr(), dstConn.RemoteAddr())

	//create channels to wait for until the forwarding of upstream and downstream data is done.
	//these are needed to enable channel waits or the defer on the source connection close() executes immediately and breaks all stream forwards
	fwd1Done := make(chan bool)
	fwd2Done := make(chan bool)
	defer close(fwd1Done)
	defer close(fwd2Done)

	//forward the source data to destination and the mirror, and destination data to the source
	//only source -> destination traffic is mirrored. not destination -> source. just add the other part if you need
	go streamFwd(srcConn, dstConn, srcConn.RemoteAddr().String()+"->"+target, fwd1Done, true)
	go streamFwd(dstConn, srcConn, target+"->"+srcConn.RemoteAddr().String(), fwd2Done, false)
	//wait until the stream forwarders exit to exit this function so the srcConn.close() is not prematurely executed
	<-fwd1Done
	<-fwd2Done
}

//streamFwd forwards a given source connection to the given destination and mirror connections
//the id parameter is used to give more meaningful prints, and the done channel to report back when the forwarding ends
func streamFwd(srcConn net.Conn, dstConn net.Conn, id string, done chan bool, upstream bool) {
	defer srcConn.Close()
	defer dstConn.Close()
	r := bufio.NewReader(srcConn)
	w := bufio.NewWriter(dstConn)

	//buffer for reading data from source and forwarding it to the destination
	//notice that a separate call to this streamFwd() function is made for src->dst and dst->src so just need one buffer and one read/write pair here
	buf := make([]byte, Config.bufferSize)

LOOPER:
	for {
		n, err := r.Read(buf)
		if n > 0 {
			//debuglog(id+": forwarding data, n=%v", n)
			_, _ = w.Write(buf[:n])
			_ = w.Flush()
			//debuglog(id + ": Write done")
		} else {
			debuglog(id+": no data received? n=%v", n)
		}
		if upstream {
			if duf != nil {
				n2, err := duf.Write(buf[:n])
				_ = duf.Sync()
				debuglog("wrote upstream data to file n=%v, err=%v", n2, err)
			}
		} else {
			if ddf != nil {
				n2, err := ddf.Write(buf[:n])
				_ = ddf.Sync()
				debuglog("wrote downstream data to file n=%v, err=%v", n2, err)
			}
		}
		//%x would print the data in hex. could be made an option or whatever
		//		debuglog("data=%x", data)

		switch err {
		case io.EOF:
			//this means the connection has been closed
			debuglog("EOF received, connection closed")
			break LOOPER
		case nil:
			//its a successful read, so lets not break the for loop but keep forwarding the stream..
			break
		default:
			//lets not crash the program on single socket error. better to wait for more connections to forward
			//log.Fatalf("Receive data failed:%s", err)
			debuglog("Breaking stream fwd due to error:%s", err)
			break LOOPER
		}

	}
	debuglog("exiting stream fwd")

	//notify the forward() function that this streamFwd() call has finished
	done <- true
}

func checkDeviceIp(deviceIp string) bool {
	testInput := net.ParseIP(deviceIp)
	if testInput.To4() == nil {
		debuglog("deviceIp(%v) is not a valid IPv4 address\n", deviceIp)
		return false
	}
	return true
}

//Configuration for the forwarder. Since it is capitalized, should be accessible outside package.
type Configuration struct {
	srcPort      int    //source where incoming connections to forward are listened to
	logFile      string //if defined, write log to this file
	logToConsole bool   //if we should log to console
	bufferSize   int    //size to use for buffering read/write data
	target       string //target ip:port
}

//this is how go defines variables, so the actual configurations are stored here
var Config Configuration

//ParseConfig reads the command line arguments and sets the global Configuration object from those. Also checks the arguments make basic sense.
func ParseConfig() {
	flagSet := flag.NewFlagSet("goforward", flag.ExitOnError)
	flagSet.SetOutput(os.Stdout)

	srcPortPtr := flagSet.Int("sp", 8080, "Source port for incoming connections. Required.")
	target := flagSet.String("target", "", "target <ip:port> for incoming connections. Required.")

	logFilePtr := flagSet.String("logf", "proxy.log", "If defined, will write debug log info to this file.")
	logToConsolePtr := flagSet.Bool("logc", false, "If defined, write debug log info to console.")
	bufferSizePtr := flagSet.Int("bufs", 1024, "Size of read/write buffering.")

	_ = flagSet.Parse(os.Args[1:])

	Config.srcPort = *srcPortPtr
	Config.logFile = *logFilePtr
	Config.logToConsole = *logToConsolePtr
	Config.bufferSize = *bufferSizePtr
	Config.target = *target

	var errors = ""
	if Config.srcPort < 1 || Config.srcPort > 65535 {
		errors += "You need to specify source port in range 1-65535.\n"
	}
	if Config.bufferSize < 1 {
		errors += "Buffer size needs to be >= 1.\n"
	}

	if len(errors) > 0 {
		println(errors)
		println("Usage: " + os.Args[0] + " [options]")
		flagSet.PrintDefaults()
		os.Exit(1)
	}
}
