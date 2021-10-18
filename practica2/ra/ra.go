/*
* AUTOR: Rafael Tolosana Calasanz
* ASIGNATURA: 30221 Sistemas Distribuidos del Grado en Ingeniería Informática
*			Escuela de Ingeniería y Arquitectura - Universidad de Zaragoza
* FECHA: septiembre de 2021
* FICHERO: ricart-agrawala.go
* DESCRIPCIÓN: Implementación del algoritmo de Ricart-Agrawala Generalizado en Go
 */
package ra

import (
	"sync"

	"../ms"
)

type Message interface{}

type Request struct {
	Clock int
	Pid   int
}

type Reply struct{}

type MessageSystem struct {
	mbox  chan Message
	peers []string
	done  chan bool
	me    int
}

type RASharedDB struct {
	OurSeqNum int
	HigSeqNum int
	OutRepCnt int
	ReqCS     bool
	RepDefd   []int
	ms        *MessageSystem
	done      chan bool
	chrep     chan bool
	Mutex     sync.Mutex // mutex para proteger concurrencia sobre las variables
	// TODO: completar
}

func New(me int, usersFile string) *RASharedDB {
	var messageTypes = []ms.Message{Request, Reply}
	msgs = ms.New(me, usersFile, messageTypes)
	ra := RASharedDB{0, 0, 0, false, []int{}, &msgs, make(chan bool), make(chan bool), &sync.Mutex{}}
	// TODO completar
	return &ra
}

//Pre: Verdad
//Post: Realiza  el  PreProtocol  para el  algoritmo de
//      Ricart-Agrawala Generalizado
func (ra *RASharedDB) PreProtocol() {
	// TODO completar
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

//Pre: True
//Post: termina la ejecución de este ms
func (ms *MessageSystem) Stop() {
	ms.done <- true
}
