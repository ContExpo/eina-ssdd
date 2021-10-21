/*
* AUTOR: Rafael Tolosana Calasanz
* ASIGNATURA: 30221 Sistemas Distribuidos del Grado en Ingeniería Informática
*			Escuela de Ingeniería y Arquitectura - Universidad de Zaragoza
* FECHA: septiembre de 2021
* FICHERO: ra.go
* DESCRIPCIÓN: Implementación del algoritmo de Ricart-Agrawala Generalizado en Go
 */
package ra

import (
	"sync"

	"prac2/ms"
)

type Request struct {
	Clock int
	Pid   int
}

type Reply struct{}

type RASharedDB struct {
	///Logical clock of the process
	OurSeqNum int
	///Highest sequence number of all processes
	HigSeqNum int
	OutRepCnt int
	ReqCS     bool
	RepDefd   []int
	ms        *ms.MessageSystem
	done      chan bool
	chrep     chan bool
	Mutex     sync.Mutex // mutex para proteger concurrencia sobre las variables
	// TODO: completar
}

func New(me int, usersFile string) *RASharedDB {
	messageTypes := []ms.Message{Request{}, Reply{}}
	var msgs = ms.New(me, usersFile, messageTypes)
	ra := RASharedDB{0, 0, 0, false, []int{}, &msgs, make(chan bool), make(chan bool), sync.Mutex{}}
	// TODO completar
	return &ra
}

//Pre: Verdad
//Post: Realiza  el  PreProtocol  para el  algoritmo de
//      Ricart-Agrawala Generalizado
func (ra *RASharedDB) PreProtocol() {

}

//Pre: Verdad
//Post: Realiza  el  PostProtocol  para el  algoritmo de
//      Ricart-Agrawala Generalizado
func (ra *RASharedDB) PostProtocol() {
	// TODO completar
}

func (ra *RASharedDB) Stop() {
	ra.ms.Stop()
	ra.done <- true
}
