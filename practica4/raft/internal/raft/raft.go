// Escribir vuestro código de funcionalidad Raft en este fichero
//

package raft

//
// API
// ===
// Este es el API que vuestra implementación debe exportar
//
// nodo = NuevoNodo(...)
//   Crear un nuevo servidor del grupo de elección.
//
// nodo.Para()
//   Solicitar la parado de un servidor
//
// nodo.ObtenerEstado() (yo, mandato, esLider)
//   Solicitar a un nodo de elección por "yo", su mandato en curso,
//   y si piensa que es el msmo el lider
//

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/rpc"
	"os"
	"sync"
)

// EnableDebugLogs false deshabilita por completo los logs de depuracion
// Aseguraros de poner EnableDebugLogs a false antes de la entrega
const EnableDebugLogs = true

// LogToStdout Poner a true para logear a stdout en lugar de a fichero
const LogToStdout = true

// LogOutputDir Cambiar esto para salida de logs en un directorio diferente
const LogOutputDir = "./logs_raft/"

// AplicaOperacion A
// medida que el nodo Raft conoce las operaciones de las  entradas de registro
// comprometidas, envía un AplicaOperacion, con cada una de ellas, al canal
// "canalAplicar" (funcion NuevoNodo) de la maquina de estados
type AplicaOperacion struct {
	indice    int // en la entrada de registro
	operacion int // el valor que almacenar
}

// Nodo Tipo de dato Go que representa un solo nodo (réplica) de raft
type Nodo struct {
	//Persistent states on all nodes
	mux      sync.Mutex            // Mutex para proteger acceso a estado compartido
	nodos    []*rpc.Client         // Conexiones RPC a todos los nodos (réplicas) Raft
	yo       int                   // this peer's index into peers[]
	logger   *log.Logger           //A logger
	Mandate  int                   //Current Mandate of election
	votedFor int                   //Who did I vote for
	leader   int                   //The leader of the Mandate. -1 means there's no leader yet
	isLeader bool                  //If the node thinks he's the leader
	opchan   *chan AplicaOperacion //The channel where to push the new operations
	// mirar figura 2 para descripción del estado que debe mantener un nodo Raft

	//Volatile state
	commitIndex int   //Highest known commit index
	lastApplied int   //Index of highest log entry applied to state machine
	ht          []int //Int array where to store the data. Functions as an hashtable with h(i) = i
}

// Creacion de un nuevo nodo de eleccion
//
// Tabla de <Direccion IP:puerto> de cada nodo incluido a si mismo.
//
// <Direccion IP:puerto> de este nodo esta en nodos[yo]
//
// Todos los arrays nodos[] de los nodos tienen el mismo orden
// canalAplicar es un canal donde, en la practica 5, se recogerán las
// operaciones a aplicar a la máquina de estados. Se puede asumir que
// este canal se consumira de forma continúa.

// NuevoNodo debe devolver resultado rápido, por lo que se deberían
// poner en marcha Gorutinas para trabajos de larga duracion
func NuevoNodo(nodos []*rpc.Client, yo int, canalAplicar *chan AplicaOperacion) *Nodo {
	nd := &Nodo{}
	nd.nodos = nodos
	nd.yo = yo

	if EnableDebugLogs {
		nombreNodo := fmt.Sprintf("%d", yo)
		logPrefix := fmt.Sprintf("%s ", nombreNodo)
		if LogToStdout {
			nd.logger = log.New(os.Stdout, nombreNodo,
				log.Lmicroseconds|log.Lshortfile)
		} else {
			err := os.MkdirAll(LogOutputDir, os.ModePerm)
			if err != nil {
				panic(err.Error())
			}
			logOutputFile, err := os.OpenFile(fmt.Sprintf("%s/%s.txt",
				LogOutputDir, logPrefix), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
			if err != nil {
				panic(err.Error())
			}
			nd.logger = log.New(logOutputFile, logPrefix, log.Lmicroseconds|log.Lshortfile)
		}
		nd.logger.Println("logger initialized")
	} else {
		nd.logger = log.New(ioutil.Discard, "", 0)
	}

	// Your initialization code here (2A, 2B)
	nd.Mandate = 1
	nd.ht = make([]int, 100)
	nd.opchan = canalAplicar
	return nd
}

// Para utilizado cuando no se necesita mas al nodo
// Quizas interesante desactivar la salida de depuracion
// de este nodo
//
func (nd *Nodo) Para() {

	for i := 0; i < len(nd.nodos); i++ {
		if i != nd.yo {
			nd.nodos[i].Go("nd.ParaNodo", &nd.yo, nil, nil)
		}
	}

}

//ParaNodo is the RPC called by the shutting note to signal the others
//about its disconnection
func (nd *Nodo) ParaNodo(nombre *int, err *error) {
	nd.mux.Lock()
	nd.nodos[*nombre].Close()
	nd.mux.Unlock()
}

// ObtenerEstado devuelve "yo", mandato en curso y si este nodo cree ser lider
//
func (nd *Nodo) ObtenerEstado() (int, int, bool) {
	return nd.yo, nd.Mandate, nd.isLeader
}

// El servicio que utilice Raft (base de datos clave/valor, por ejemplo)
// Quiere buscar un acuerdo de posicion en registro para siguiente operacion
// solicitada por cliente.

// SometerOperacion si el nodo no es el lider, devolver falso
// Sino, comenzar la operacion de consenso sobre la operacion y devolver con
// rapidez
//
// No hay garantia que esta operacion consiga comprometerse en una entrada de
// de registro, dado que el lider puede fallar y la entrada ser reemplazada
// en el futuro.
// Primer valor devuelto es el indice del registro donde se va a colocar
// la operacion si consigue comprometerse.
// El segundo valor es el mandato en curso
// El tercer valor es true si el nodo cree ser el lider
func (nd *Nodo) SometerOperacion(operacion interface{}) (indice int,
	mandato int, isLeader bool) {
	if !nd.isLeader {
		return -1, nd.Mandate, false
	}

	// Vuestro codigo aqui

	return indice, nd.Mandate, isLeader
}

// RequestVoteArgs struct to send a VoteRequest
type RequestVoteArgs struct {
	CandidateID    int //The candidate asking for vote
	Mandate        int //The Mandate for which we ask the vote for
	LastLogIndex   int
	LastLogMandate int
}

//
// RequestVoteReply struct to send an answer to a vote request
type RequestVoteReply struct {
	Mandate     int  //Mandate of the receiving nd
	voteGranted bool //If the vote was granted
}

// NewLeaderCallArgs struct to send a message to others to communicate being
//the new leader
type NewLeaderCallArgs struct {
	Leader  int //Who is communicating to be the new leader
	Mandate int //The Mandate for which we ask the vote for
}

//HeartBeat is a keep-alive message the leader sends to its followers to
//confirm them its well-being
type HeartBeat NewLeaderCallArgs

//
// PedirVoto RPC method to ask to the other actors a vote to choose a new leader
// ===========
//
// Metodo para RPC PedirVoto
//
func (nd *Nodo) PedirVoto(args *RequestVoteArgs, reply *RequestVoteReply) {
	if args.Mandate > nd.Mandate {
		nd.mux.Lock()
		nd.Mandate = args.Mandate
		nd.votedFor = -1
		nd.mux.Unlock()
	}
	var votedForMe = 0
	//Send a msg to all the nodes asking for elections
	//We do it to all since the "heuristic" for deciding the next leader
	//won't be just "highest number"
	for i := 0; i < len(nd.nodos); i++ {
		var reqVoteArgs = RequestVoteArgs{
			CandidateID: nd.yo,
			Mandate:     nd.Mandate,
		}
		var reqVoteReply = RequestVoteReply{}
		if nd.enviarPeticionVoto(i, &reqVoteArgs, &reqVoteReply) {
			if reqVoteReply.Mandate > nd.Mandate {
				nd.updateMandate(reqVoteReply.Mandate)
				return
			}
			if reqVoteReply.voteGranted &&
				reqVoteReply.Mandate == nd.Mandate {
				votedForMe++
				if votedForMe > len(nd.nodos)/2 {
					nd.isLeader = true
				}
				//I am the leader
				go nd.sendHeartbeats()
			}
		}
	}
}

//
// sendRequestVote
// ===============
//
// Example code to send a RequestVote RPC to a server
//
// server int -- index of the target server in
// rf.peers[]
//
// args *RequestVoteArgs -- RPC arguments in args
//
// reply *RequestVoteReply -- RPC reply
//
// The types of args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers)
//
// The rpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost
//
// Call() sends a request and waits for a reply
//
// If a reply arrives within a timeout interval, Call() returns true;
// otherwise Call() returns false
//
// Thus Call() may not return for a while
//
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply
//
// Call() is guaranteed to return (perhaps after a delay)
// *except* if the handler function on the server side does not return
//
// Thus there
// is no need to implement your own timeouts around Call()
//
// Please look at the comments and documentation in ../rpc/rpc.go
// for more details
//
// If you are having trouble getting RPC to work, check that you have
// capitalized all field names in the struct passed over RPC, and
// that the caller passes the address of the reply struct with "&",
// not the struct itself
//
// The method will not perform an RPC call to itself, skipping it if nodo
// equals nd.yo, returning true (cause technically the node already knows that
// a new election is needed)
func (nd *Nodo) enviarPeticionVoto(nodo int, args *RequestVoteArgs,
	reply *RequestVoteReply) bool {
	if nd.yo == nodo {
		reply.Mandate = nd.Mandate
		reply.voteGranted = true
		nd.mux.Lock()
		nd.votedFor = nd.yo
		nd.mux.Unlock()
		return true
	}
	ok := nd.nodos[nodo].Call("Nodo.PedirVoto", args, reply)

	return ok == nil
}

//SetNewLeader is an RPC call to the other peers to be performed when a node is
//sure to be the new leader because he has >50% of the votes
//The reply parameter is the current Mandate of the receiving node
func (nd *Nodo) SetNewLeader(args *NewLeaderCallArgs,
	reply *int) bool {
	*reply = nd.Mandate
	if nd.Mandate > args.Mandate {
		return false
	}

	nd.mux.Lock()
	nd.leader = args.Leader
	nd.Mandate = args.Mandate
	nd.mux.Unlock()
	return true
}

func (nd *Nodo) updateMandate(Mandate int) {
	if nd.Mandate < Mandate {
		nd.mux.Lock()
		nd.Mandate = Mandate
		nd.votedFor = -1
		nd.isLeader = false
		nd.leader = -1
		nd.mux.Unlock()
	}
}

//sendHeartbeats needs to be invoked as a Goroutine and it will
//send keep-alive messages to the followers as long as nd.isLeader is true.
//when it detects it's turned false (for instance by *Nodo.updateMandate)
//it will stop
func (nd *Nodo) sendHeartbeats() {
	var hb = HeartBeat{nd.yo, nd.Mandate}
	for i := 0; nd.isLeader; i++ {
		if i != nd.yo {
			nd.nodos[i].Go("Nodo.KeepAlive", &hb, nil, nil)
		}
	}
}

//
//KeepAlive is the function called by the leader in the followers node to
//signal its wellbeing.
func (nd *Nodo) KeepAlive() {
	var hb = HeartBeat{nd.yo, nd.Mandate}
	for i := 0; nd.isLeader; i++ {
		if i != nd.yo {
			nd.nodos[i].Go("nd.KeepAlive", &hb, nil, nil)
		}
	}
}
