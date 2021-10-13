package main

import (
	"bufio"
	"crypto/x509"
	"encoding/gob"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"prac1/com"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type SshClient struct {
	Config *ssh.ClientConfig
	Server string
}

func NewSshClient(user string, host string, port int, privateKeyPath string, privateKeyPassword string) (*SshClient, error) {
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

	client := &SshClient{
		Config: config,
		Server: fmt.Sprintf("%v:%v", host, port),
	}

	return client, nil
}

// Opens a new SSH connection and runs the specified command
// Returns the combined output of stdout and stderr
func (s *SshClient) RunCommand(cmd string) (string, error) {
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
	output, err := session.CombinedOutput(cmd)

	return fmt.Sprintf("%s", output), err
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

func manageWorker(address string, channel *chan com.Request, encoder *gob.Encoder) {

	//Port is the port that the client will open a connection on, serverPort is the dedicated port to that connection from the server
	fmt.Println("Trying to connect to worker at " + address)
	split := strings.Split(address, ":")
	port, err := strconv.Atoi(split[1])
	checkError(err)
	var username = "conte"
	ssh, err := NewSshClient(
		username,
		split[0],
		22,
		"/home/"+username+"/.ssh/id_rsa",
		"")
	if err != nil {
		fmt.Printf("Worker %v didn't respond, terminating proxy to it\n", address)
		checkError(err)
		return
	}

	var command string
	command = fmt.Sprintf("./workerprac1 %d", port)
	fmt.Println("Executing command " + command)
	resp, err := ssh.RunCommand(command)
	checkError(err)
	fmt.Println("Worker resp: " + resp)
	time.Sleep(1000 * time.Millisecond) //Give time to client to set up
	var endpoint = fmt.Sprintf("127.0.0.1:%d", port)
	tcpAddr, err := net.ResolveTCPAddr("tcp", endpoint)
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	fmt.Println("Dialing " + tcpAddr.String())
	checkError(err)

	var workerEncoder = gob.NewEncoder(conn)
	var workerDecoder = gob.NewDecoder(conn)
	checkError(err)
	//Command to run is command clientport
	var reply com.Reply
	var req com.Request
	for {
		req = <-*channel
		checkError(err)
		workerEncoder.Encode(req)
		err = workerDecoder.Decode(&reply)
		//fmt.Println("Sending: ", reply)
		encoder.Encode(reply)
	}
}

func main() {

	fmt.Println("Waiting for connection")
	listener, err := net.Listen("tcp", "127.0.0.1:30000")
	//var freePort = 30001
	checkError(err)
	conn, err := listener.Accept()
	defer conn.Close()
	checkError(err)
	encoder := gob.NewEncoder(conn)
	decoder := gob.NewDecoder(conn)
	var reqchan = make(chan com.Request, 20)
	file, err := os.Open("workers.txt")
	checkError(err)
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		go manageWorker(scanner.Text(), &reqchan, encoder)
		//freePort = freePort + 1
	}
	var req com.Request
	fmt.Println("Server on\n")
	for {
		err := decoder.Decode(&req)
		checkError(err)
		reqchan <- req
	}
}
