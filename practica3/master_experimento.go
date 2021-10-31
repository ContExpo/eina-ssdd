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

func startWorker(address string) {

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
	fmt.Println("Ended creating ssh object")
	if err != nil {
		fmt.Printf("Worker %v didn't respond, terminating proxy to it\n", address)
		checkError(err)
		return
	}
	var command string
	if username == "conte" {
		command = fmt.Sprintf("go run ./eina-ssdd/practica3/worker.go %d", port)
	} else if username == "a847803" {
		command = fmt.Sprintf("./worker %d", port)
	}
	fmt.Println("Executing through ssh ", command)
	go ssh.RunCommand(command)
	checkError(err)
	//Putting the new worker in the address
	freeWorkers <- address
}

func (p *PrimesImpl) FindPrimes(interval com.TPInterval, primeList *[]int) error {
	//By doing this, we will keep
	for true {
		var endpoint = <-freeWorkers
		client, err := rpc.DialHTTP("tcp", endpoint)
		if err != nil {
			//If there's an issue calling the worker I try another one and
			//send its address to the function that will try to restart it
			go startWorker(endpoint)
			continue
		}
		err = client.Call("PrimesImpl.FindPrimes", interval, primeList)
		if err != nil {
			//Same as above: worker gives error, try restart and use another
			go startWorker(endpoint)
			continue
		}
		//If I managed to get till here with no errors it means RPC call was
		//good. I can break out of loop and return null error.
		//*primeList will contain the prime numbers.
		break
	}
	return nil
}

/*We're using this global channel in FindPrimes to pass it to the FindPrimes
asynchronous worker execution so we can prevent delays and crashes. After
5 seconds we will look in the channel and if the call is not completed we will
remake it or eventually restart client*/
var done chan rpc.Call

/*A global channel where the master can pull out workers to call, and when he's
done push them again in the queue. If the worker dies and can't be restarted
it won't be put back in */
var freeWorkers chan string

func main() {
	var endpoint string
	if len(os.Args) == 1 {
		endpoint = "127.0.0.1:30000"
	} else {
		endpoint = os.Args[1]
	}
	fmt.Println("Opening server on ", endpoint)
	file, err := os.Open("workers.txt")
	checkError(err)
	defer file.Close()
	scanner := bufio.NewScanner(file)
	freeWorkers = make(chan string, 100)
	var i = 0
	for scanner.Scan() {
		go startWorker(scanner.Text())
		i++
	}
	done = make(chan rpc.Call, i)
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
