/*
* AUTOR: Alessio Esposito Inchiostro
* ASIGNATURA: 30221 Sistemas Distribuidos del Grado en Ingeniería Informática
*			Escuela de Ingeniería y Arquitectura - Universidad de Zaragoza
* FECHA: octobre de 2021
* FICHERO: reader.go
* DESCRIPCIÓN: Implementation of a reader that will interact asyncronously with
* 			   other readers and writers using Ricard-Agrawala algorithm
 */

package main

import (
	"prac2/reader"
	"prac2/writer"
)

func main() {
	var readersCount = 3
	var writersCount = 3
	var readers [3]*reader.Reader
	var writers [3]*writer.Writer
	for i := 0; i < readersCount; i++ {
		readers[i] = reader.New(i, 0, "peers.txt")
	}
	for i := 0; i < writersCount; i++ {
		writers[3+i] = writer.New(i, 0, "peers.txt")
	}
}
