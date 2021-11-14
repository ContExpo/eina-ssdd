package main

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	rpctimeout "raft/internal/comun/rpc"
	"time"
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
	arith := new(Arith)
	// Parte Servidor
	rpc.Register(arith)

	l, e := net.Listen("tcp", ":1234")
	if e != nil {
		log.Fatal("listen error:", e)
	}

	// Quitar el lanzamiento de la gorutina, pero no el código interno.
	// Solo se necesita para esta prueba dado que cliente y servidor estan,
	// aqui, juntos
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				continue
			}

			go rpc.ServeConn(conn)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// Parte Cliente
	client, err := rpc.Dial("tcp", "127.0.0.1:1234")
	if err != nil {
		log.Fatal("dialing:", err)
	}

	var reply int
	args := Args{5, 7}
	err = rpctimeout.CallTimeout(client, "t.Mul", &args,
		&reply, 5*time.Millisecond)

	if err != nil {
		log.Fatal("arith error:", err)
	}

	fmt.Printf("Arith: %d * %d = %d", args.A, args.B, reply)
}
