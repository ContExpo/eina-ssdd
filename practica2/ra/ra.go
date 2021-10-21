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
	"reflect"
	"sync"

	"prac2/ms"
)

type Request struct {
	Clock int
	Pid   int
}

type Reply struct {
	//Pid of the process which is giving authorization to enter critical section
	Pid int
}

type RASharedDB struct {
	//Logical clock of the process
	OurSeqNum int
	//Highest sequence number of all processes
	HigSeqNum int
	OutRepCnt int
	ReqCS     bool
	//Array of Pids who are waiting for authorization from the client to enter critical section
	RepDefd []int
	//Message system
	ms *ms.MessageSystem
	//If the system is to be shut down
	done  chan bool
	chrep chan bool
	//Mutex para proteger concurrencia sobre las variables
	Mutex sync.Mutex
	//The count of all the peers in the system excluding the current
	//(Eg in a system with 10 clients PeersCount will be 9)
	PeersCount int
}

func New(me int, peers int, usersFile string) *RASharedDB {
	messageTypes := []ms.Message{Request{}, Reply{}}
	var ms = ms.New(me, usersFile, messageTypes)

	ra := RASharedDB{0, 0, 0, false, []int{}, &ms, make(chan bool), make(chan bool), sync.Mutex{}, peers}
	// TODO completar
	return &ra
}

//Pre: Verdad
//Post: Realiza  el  PreProtocol  para el  algoritmo de
//      Ricart-Agrawala Generalizado
func (ra *RASharedDB) PreProtocol() {
	ra.Mutex.Lock()
	//Increasing our sequence number as asking for a critical section is an event
	ra.OurSeqNum++
	for i := 0; i < ra.PeersCount; i++ {
		if i != ra.ms.Me {
			//Send all the requests
			ra.ms.Send(i, Request{ra.OurSeqNum, ra.ms.Me})
			//TODO ask the professor if I could write ms.SendAll?
		}
	}
	ra.Mutex.Unlock()
	//After unlocking the Mutex, wait for the authorization from all the peers
	ra.waitForAuthorization()
}

func (ra *RASharedDB) waitForAuthorization() {
	for count := ra.PeersCount; count != 0; {
		var msg = ra.ms.Receive()
		var msgType = reflect.TypeOf(msg)
		if msgType == reflect.TypeOf(Reply{}) {
			count--
		}
	}
}

//Pre: Verdad
//Post: Realiza  el  PostProtocol  para el  algoritmo de
//      Ricart-Agrawala Generalizado
func (ra *RASharedDB) PostProtocol() {
	ra.Mutex.Lock()
	//For each deferred auth to enter critical section I give permission
	//cause I'm done
	for pid := range ra.RepDefd {
		ra.ms.Send(pid, Reply{ra.ms.Me})
	}
	ra.Mutex.Unlock()
}

func (ra *RASharedDB) Stop() {
	ra.ms.Stop()
	ra.done <- true
}
