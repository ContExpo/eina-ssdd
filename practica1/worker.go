package main

import (
	"fmt"
	"os"
	"prac1/com"
	"strconv"
)

func checkError(err error) {
	if err != nil {
		//fmt.Fprintf(os.Stderr, "Fatal error: %s", err.Error())
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

func PrintPrimes(interval com.TPInterval) {
	for i := interval.A; i <= interval.B; i++ {
		if IsPrime(i) {
			fmt.Printf("%d ", i)
		}
	}
}

/*Servidor que recibe un intervalo de un cliente y le devuelve
los primos dentro del intervalo
*/
func main() {

	if len(os.Args) != 3 {
		return
	}
	var first int
	var last int
	var err error
	first, err = strconv.Atoi(os.Args[1])
	checkError(err)
	last, err = strconv.Atoi(os.Args[2])
	checkError(err)
	PrintPrimes(com.TPInterval{first, last})
}
