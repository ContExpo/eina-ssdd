package main

import (
	"bufio"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"prac3/com"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

//PrimesImpl is the struct used to keep track of the metrics of FindPrimes
type PrimesImpl struct {
	delayMaxMilisegundos int
	delayMinMiliSegundos int
	behaviourPeriod      int
	behaviour            int
	i                    int
	mutex                sync.Mutex
	//The channel where FindPrimes will push new requests
	reqchan *chan ClientRequest
	//In case a request cannot be handled cause of malfunctioning worker
	//it will be put here to increase its priority
	hiPrioReqchan *chan ClientRequest
}

//ClientRequest represents the struct passed to the goroutines to execute
//FindPrimes for
type ClientRequest struct {
	interval com.TPInterval
	reschan  *chan []int
}

//SSHClient struct
type SSHClient struct {
	Config *ssh.ClientConfig
	Server string
}

//NewSSHClient generates a new SSH client to use
func NewSSHClient(user string, host string, port int, privateKeyPath string, privateKeyPassword string) (*SSHClient, error) {
	// read private key file
	pemBytes, err := ioutil.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("Reading private key file failed %v", err)
	}
	// create signer
	signer, err := signerFromPem(pemBytes, []byte(privateKeyPassword))
	if err != nil {
		return nil, err
	}
	// build SSH client config
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
			ssh.Password("Arduino"),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			// use OpenSSH's known_hosts file if you care about host validation
			return nil
		},
	}

	client := &SSHClient{
		Config: config,
		Server: fmt.Sprintf("%v:%v", host, port),
	}

	return client, nil
}

func signerFromPem(pemBytes []byte, password []byte) (ssh.Signer, error) {

	// read pem block
	err := errors.New("Pem decode failed, no key found")
	pemBlock, _ := pem.Decode(pemBytes)
	if pemBlock == nil {
		return nil, err
	}

	// handle encrypted key
	if x509.IsEncryptedPEMBlock(pemBlock) {
		// decrypt PEM
		pemBlock.Bytes, err = x509.DecryptPEMBlock(pemBlock, []byte(password))
		if err != nil {
			return nil, fmt.Errorf("Decrypting PEM block failed %v", err)
		}

		// get RSA, EC or DSA key
		key, err := parsePemBlock(pemBlock)
		if err != nil {
			return nil, err
		}

		// generate signer instance from key
		signer, err := ssh.NewSignerFromKey(key)
		if err != nil {
			return nil, fmt.Errorf("Creating signer from encrypted key failed %v", err)
		}

		return signer, nil
	} else {
		// generate signer instance from plain key
		signer, err := ssh.ParsePrivateKey(pemBytes)
		if err != nil {
			return nil, fmt.Errorf("Parsing plain private key failed %v", err)
		}

		return signer, nil
	}
}

func parsePemBlock(block *pem.Block) (interface{}, error) {
	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("Parsing PKCS private key failed %v", err)
		} else {
			return key, nil
		}
	case "EC PRIVATE KEY":
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("Parsing EC private key failed %v", err)
		} else {
			return key, nil
		}
	case "DSA PRIVATE KEY":
		key, err := ssh.ParseDSAPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("Parsing DSA private key failed %v", err)
		} else {
			return key, nil
		}
	default:
		return nil, fmt.Errorf("Parsing private key failed, unsupported key type %q", block.Type)
	}
}

func checkError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %s", err.Error())
		os.Exit(1)
	}
}

// Opens a new SSH connection and runs the specified command
// Returns the combined output of stdout and stderr
// @param clientRef where to put the reference to the new client
func (s *SSHClient) runCommand(cmd string, clientRef **ssh.Client) (string, error) {
	// open connection
	conn, err := ssh.Dial("tcp", s.Server, s.Config)
	if err != nil {
		return "", fmt.Errorf("Dial to %v failed %v", s.Server, err)
	}
	defer conn.Close()

	// open session
	session, err := conn.NewSession()
	if err != nil {
		return "", fmt.Errorf("Create session for %v failed %v", s.Server, err)
	}
	defer session.Close()

	// run command and capture stdout/stderr
	fmt.Println("Executing command " + cmd)
	output, err := (session).CombinedOutput(cmd)
	var resp = fmt.Sprintf("%s", output)
	fmt.Println("Server answer: " + resp + "---")
	return resp, err
}

func manageWorker(address string, primesImpl *PrimesImpl) {

	//Port is the port that the client will open a connection on, serverPort is the dedicated port to that connection from the server
	fmt.Println("Trying to connect to worker at " + address)
	split := strings.Split(address, ":")
	port, err := strconv.Atoi(split[1])
	checkError(err)
	var username = "conte"
	sshConfig, err := NewSSHClient(
		username,
		split[0],
		22,
		"/home/a847803/.ssh/id_rsa",
		"")
	fmt.Println("Ended creating ssh object")
	if err != nil {
		fmt.Printf("Worker %v didn't respond, terminating proxy to it\n", address)
		checkError(err)
		return
	}

	var command string
	if username == "conte" {
		command = fmt.Sprintf("go run ./eina-ssdd/practica1/worker.go %d", port)
	} else if username == "a847803" {
		command = fmt.Sprintf("./worker %d", port)
	}
	var sshClient *ssh.Client
	go sshConfig.runCommand(command, &sshClient)
	//fmt.Println(resp)
	checkError(err)
	//I know that the worker needs 10 secs to set up, so
	time.Sleep(time.Second * 10)
	rpcClient, err := rpc.DialHTTP("tcp", address)
	if err != nil {
		log.Fatal("Error dialing:", err)
	}
	var reply []int
	var clientReq ClientRequest
	for {
		/*Infinite loop of a goroutine. First we check if any high priority
		request is waiting for another goroutine to elaborate it. Otherwise we
		read from the default queue
		*/
		if len(*primesImpl.hiPrioReqchan) > 0 {
			clientReq = <-*primesImpl.hiPrioReqchan
		} else {
			clientReq = <-*primesImpl.reqchan
		}

		/*Making asynchronous call to the client. As for documentation, the only
		way for the call not to be completed is if the client invoked RPC does NOT
		return. For example in a case of omission.*/
		var rpcReq = rpcClient.Go("PrimesImpl.FindPrimes", clientReq, &reply, nil)

		/*I give the client 1.5 seconds in which he can terminate the call. If
		it doesn't happen and rpc.Go doesn't return an error, it's either a
		delay or a omission. In this case I pass the tarea to another goroutine
		and will decide what to do.
		In total, there is 20% chance to have a delay and another 20 to have an omission.
		Let's suppose time wasted for a delay is 6 secs (that will be the avg
		time that the client will need to respond)), while time used for a restart
		will be 17.5 seconds (7.5 secs to see if the client is alive and then 10 to
		restart the client). We need to choose what's the best way to handle these 2
		errors, keeping in mind that there are equal chances that those 2 will happen
		*/
		for i := 0; i < 20; i++ {
			time.Sleep(100 * time.Millisecond)
			if len(rpcReq.Done) == 0 {
				continue
			} else {
				err = rpcReq.Error
				//If the worker has some error
				if err != nil {
					//I push the request in the high prio channel
					*primesImpl.hiPrioReqchan <- clientReq
					//Closing the ssh client to restart it
					sshClient.Close()
					go manageWorker(address, primesImpl)
					return
				}
				*clientReq.reschan <- reply
				//Since we need to create a new one, we make sure the previous client is closed.
				sshClient.Close()
				return
			}
		}
		/*if we got here it's because after 2 seconds the worker has not responded nor terminated with an
		error. At this point we assume it's either a delay or an omission -> our strategy makes us restart
		the worker now and push the request in the hi prio channel */
		*primesImpl.hiPrioReqchan <- clientReq
		go manageWorker(address, primesImpl)
		return
	}
}

//FindPrimes gets invoked remotely by client/proxy and passes the result to the goroutines
func (p *PrimesImpl) FindPrimes(interval com.TPInterval, primeList *[]int) error {
	var reschan = make(chan []int)
	*p.reqchan <- ClientRequest{interval, &reschan}
	var resp = <-reschan
	primeList = &resp
	return nil
}

func main() {

	var endpoint string
	if len(os.Args) == 1 {
		endpoint = "127.0.0.1:30000"
	} else {
		endpoint = os.Args[1]
	}
	fmt.Println("Opening server on ", endpoint)

	var reqchan = make(chan ClientRequest, 20)
	var hiPrioReqchan = make(chan ClientRequest, 20)

	file, err := os.Open("workers.txt")
	checkError(err)
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var i = 0
	primesImpl := new(PrimesImpl)
	primesImpl.delayMaxMilisegundos = 4000
	primesImpl.delayMinMiliSegundos = 2000
	primesImpl.behaviourPeriod = 4
	primesImpl.i = 1
	primesImpl.behaviour = 0
	primesImpl.reqchan = &reqchan
	primesImpl.hiPrioReqchan = &hiPrioReqchan
	for scanner.Scan() {
		go manageWorker(scanner.Text(), primesImpl)
		i++
	}
	rpc.Register(primesImpl)
	rpc.HandleHTTP()
	l, e := net.Listen("tcp", endpoint)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	http.Serve(l, nil)
	fmt.Println("Waiting for connection")

}
