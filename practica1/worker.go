package main

import (
	"encoding/gob"
	"fmt"
	"net"
	"os"
	"prac1/com"
	"time"
)

func checkError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %s", err.Error())
		os.Exit(1)
	}
}

// PRE: verdad
// POST: IsPrime devuelve verdad si n es primo y falso en caso contrario
func IsPrime(n int) (foundDivisor bool) {
	foundDivisor = false
	for i := 2; (i < n) && !foundDivisor; i++ {
		foundDivisor = (n%i == 0)
	}
	return !foundDivisor
}

// PRE: interval.A < interval.B
// POST: FindPrimes devuelve todos los números primos comprendidos en el
// 		intervalo [interval.A, interval.B]
func FindPrimes(interval com.TPInterval) (primes []int) {
	for i := interval.A; i <= interval.B; i++ {
		if IsPrime(i) {
			primes = append(primes, i)
		}
	}
	return primes
}

func closeAfter(seconds int) {
	time.Sleep(120 * time.Second)
	os.Exit(1)
}

/*Servidor que recibe un intervalo de un cliente y le devuelve
los primos dentro del intervalo
*/
func main() {
	// closeAfter(120)
	if len(os.Args) != 2 {
		fmt.Println("Wrong arguments")
		return
	}

	var err error
	CONN_TYPE := "tcp"
	//CONN_HOST := "0.0.0.0"
	CONN_PORT := os.Args[1]

	fmt.Println("hello")

	//Abrimos puerto
	listener, err := net.Listen(CONN_TYPE, ":"+CONN_PORT)
	checkError(err)
	fmt.Println("Waiting for client")

	//Aceptamos conexion
	conn, err := listener.Accept()
	fmt.Println("Client found")
	checkError(err)
	defer conn.Close()

	/*Creamos un codificador para mandar respuestas serializadas
	por gob y un decodificador para recibir peticiones y
	deserializarlas
	*/
	encoder := gob.NewEncoder(conn)
	decoder := gob.NewDecoder(conn)

	var reply com.Reply
	var req com.Request

	for {
		//Recibimos la peticion
		fmt.Println("Awaiting request")
		err = decoder.Decode(&req)
		//fmt.Println("Request received")
		checkError(err)
		reply.Id = req.Id

		//Calculamos los primos que devolver
		reply.Primes = FindPrimes(req.Interval)

		//Enviamos el intervalo
		err = encoder.Encode(reply)
		checkError(err)
	}
}
