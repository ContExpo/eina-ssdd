package main

import (
	"bufio"
	"fmt"
	"log"
	"net/rpc"
	"os"
	"raft/internal/despliegue"
	"raft/internal/raft"
)

//Args some arguments
type Args struct {
	A, B int
}

//Arith An int
type Arith int

//Mul RPC func to multiply
func (t *Arith) Mul(args *Args, reply *int) error {
	*reply = args.A * args.B
	return nil
}

func main() {

	mydir, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(mydir)
	var rpcConns = startNodes()
	var nodos [3]*raft.Nodo
	for i := 0; i < 3; i++ {
		nodos[i] = raft.NuevoNodo(rpcConns, i, nil, i*3)
	}
}

func startNodes() (nodos []*rpc.Client) {
	var hosts []string
	file, err := os.Open("workers.txt")
	checkError(err)
	defer file.Close()
	scanner := bufio.NewScanner(file)

	var i = 0
	for scanner.Scan() {
		hosts = append(hosts, scanner.Text())
		i++
	}

	var username = "conte"
	var path = "/home/" + username + "/.ssh/id_rsa"

	despliegue.ExecMutipleNodes("./nodep4", hosts, nil, path)

	for j := 0; j < len(hosts); j++ {
		nodos[i], err = rpc.DialHTTP("tcp", hosts[i])
		if err != nil {
			log.Fatal("Error dialing:", err)
		}
	}
	return nodos
}

func checkError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %s", err.Error())
		os.Exit(1)
	}
}
