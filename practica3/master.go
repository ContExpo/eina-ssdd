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

	"golang.org/x/crypto/ssh"
)

type PrimesImpl struct {
	delayMaxMilisegundos int
	delayMinMiliSegundos int
	behaviourPeriod      int
	behaviour            int
	i                    int
	mutex                sync.Mutex
}

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
	fmt.Println("Executing command " + cmd)
	output, err := session.CombinedOutput(cmd)
	var resp = fmt.Sprintf("%s", output)
	fmt.Println("Server answer: " + resp + "---")
	return resp, err
}

func manageWorker(address string) {

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

	go ssh.RunCommand(command)
	//fmt.Println(resp)
	checkError(err)

	client, err := rpc.DialHTTP("tcp", address)
	if err != nil {
		log.Fatal("Error dialing:", err)
	}
	var reply *[]int
	for req := range reqchan {
		//3) la goroutine que coja la tarea invoca por RPC al worker
		//4) la goroutine le devuelve el resultado al master.
		err = client.Call("PrimesImpl.FindPrimes", req, &reply)
		//aquì voy a gestionar los varios errores que se pueden encontrar
		if err != nil {
			log.Fatal("primes error:", err)
		}
		//Now I've gotten the result of the call. How do I return them to the
		//FindPrimes??
		reschan <- reply
		//I could use an asynchronous channel like this.
		//But let's say I have 5 FindPrimes waiting for an answer. Nobody
		//assures me the right one will get the answer
	}

}

func (p *PrimesImpl) FindPrimes(interval com.TPInterval, primeList *[]int) error {
	reqchan <- interval
	//1) poner la tarea en el canal sincrono donde escuchan las gorutines OK
	//2) el master se queda bloqueado a la espera de obtener el resultado
	primeList = <-reschan
	//As I said, I have no way to be sure this is the right answer to the client's
	//request
	return nil
}

//The two synchronous channels
var reqchan chan com.TPInterval
var reschan chan *[]int

func main() {

	var endpoint string
	if len(os.Args) == 1 {
		endpoint = "127.0.0.1:30000"
	} else {
		endpoint = os.Args[1]
	}
	fmt.Println("Opening server on ", endpoint)

	reqchan = make(chan com.TPInterval)
	reschan = make(chan *[]int)

	file, err := os.Open("workers.txt")
	checkError(err)
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var i = 0
	for scanner.Scan() {
		go manageWorker(scanner.Text())
		i++
	}
	primesImpl := new(PrimesImpl)
	primesImpl.delayMaxMilisegundos = 4000
	primesImpl.delayMinMiliSegundos = 2000
	primesImpl.behaviourPeriod = 4
	primesImpl.i = 1
	primesImpl.behaviour = 0
	rpc.Register(primesImpl)
	rpc.HandleHTTP()
	l, e := net.Listen("tcp", endpoint)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	http.Serve(l, nil)
	fmt.Println("Waiting for connection")

}
