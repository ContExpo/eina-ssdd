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
	rpctimeout "raft/internal/comun/rpc"
	"sync"
	"time"
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
	mux           sync.Mutex            // Mutex para proteger acceso a estado compartido
	nodos         []*rpc.Client         // Conexiones RPC a todos los nodos (réplicas) Raft
	yo            int                   // this peer's index into peers[]
	logger        *log.Logger           //A logger
	currentTerm   int                   //Current term of election
	votedFor      int                   //Who did I vote for
	currentLeader int                   //The node believed to be the leader for the current Term
	isLeader      bool                  //If the node thinks he's the leader
	opchan        *chan AplicaOperacion //The channel where to push the new operations
	ht            []Operacion           //Int array where to store the data. Functions as an hashtable with h(i) = i
	lastHeartbeat time.Time             //Needed to track last received heartbeat
	sleepTime     int                   //Period of time of "not contact" needed to start an election
	sleepTimeStr  string                //String of sleep time needed for duration

	//Volatile state for all
	thisCommitIndex int //Highest known commit index
	thisLastApplied int //Index of highest log entry applied to state machine
	//Volatile state for leader
	nextIndex   []int //Currently known highest entries in other nodes
	commitIndex []int //Currently known index of the highest committed op in the other nodes
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
func NuevoNodo(nodos []*rpc.Client, yo int,
	canalAplicar *chan AplicaOperacion,
	sleepTime int) *Nodo {
	nd := &Nodo{}
	nd.nodos = nodos
	nd.yo = yo
	nd.thisCommitIndex = 0
	nd.isLeader = false
	nd.thisLastApplied = 0
	for i := 0; i < len(nd.commitIndex); i++ {
		nd.commitIndex[i] = 0
		nd.nextIndex[i] = 0
	}
	nd.votedFor = -1
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
	nd.currentTerm = 1
	nd.ht = make([]Operacion, 100)
	nd.opchan = canalAplicar

	nd.sleepTime = sleepTime
	nd.sleepTimeStr = fmt.Sprintf("%ds", sleepTime)
	go nd.heartbeatListener()
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
	return nd.yo, nd.currentTerm, nd.isLeader
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
func (nd *Nodo) SometerOperacion(operacion AplicaOperacion) (indice int,
	mandato int, isLeader bool) {
	if !nd.isLeader {
		return -1, nd.currentTerm, false
	}
	*nd.opchan <- operacion
	return indice, nd.currentTerm, isLeader
}

//CommitRequest is the structure sent by the leader to the followers
// to commit operations
type CommitRequest struct {
	Op       Operacion
	Index    int
	LeaderID int
	Term     int
}

//CommitAnswer is the struct returned by a follower node when it receives a
//commit request
type CommitAnswer struct {
	Success bool
	Term    int
}

//ComprometerOperacion es la funciòn RPC llamada por el master en los seguidores
//para intentar de comprometer una operacion
func (nd *Nodo) ComprometerOperacion(args *CommitRequest, ans *CommitAnswer) {
	nd.mux.Lock()
	nd.lastHeartbeat = time.Now()
	defer nd.mux.Unlock()
	if nd.currentTerm > args.Term {
		ans.Term = nd.currentTerm
		ans.Success = false
		return
	}
	nd.currentTerm = args.Term
	nd.currentLeader = args.LeaderID
	nd.ht[args.Index] = args.Op
	ans.Success = true
}

// Operacion is the struct used to represent an operation saved in the state
// machine
// ===============
// Structura de ejemplo de argumentos de RPC PedirVoto.
//
// Recordar
// -----------
// Nombres de campos deben comenzar con letra mayuscula !
//
type Operacion struct {
	Value int //Value of the operation to save
	N_ACK int //The amount of follower nodes that acknowledged the operation
}

// RequestVoteArgs struct to send a VoteRequest
// ===============
// Structura de ejemplo de argumentos de RPC PedirVoto.
//
// Recordar
// -----------
// Nombres de campos deben comenzar con letra mayuscula !
//
type RequestVoteArgs struct {
	Term        int
	CandidateID int
}

//
// RequestVoteReply struct to send an answer to a vote request
// ================
type RequestVoteReply struct {
	Term        int
	VoteGranted bool
}

//EmpezarElecion es llamada por heartbeatListener cuando se da cuenta que
//el leader està en timeout
func (nd *Nodo) EmpezarElecion() {
	timeout, err := time.ParseDuration("500ms")
	if err != nil {

	}

	nd.mux.Lock()
	var term = nd.currentTerm + 1
	nd.mux.Unlock()
	nd.updateTerm(term, false)

	nd.mux.Lock()

	//number of other notes that voted for me
	var votedForMe = 0
	for index, node := range nd.nodos {
		//I need to use a break because the range expression won't allow
		//a condition to stop the loop

		//To ignore unused variable
		_ = index

		var requestVoteArgs = RequestVoteArgs{Term: nd.currentTerm,
			CandidateID: nd.yo}
		var response = RequestVoteReply{}
		var error = rpctimeout.CallTimeout(node, "nd.PedirVoto", &requestVoteArgs,
			&response, timeout)
		if error != nil {
			//If I find a node with a higher mandate than me I quit the election
			if nd.currentTerm < response.Term {
				nd.mux.Unlock()
				nd.updateTerm(response.Term, false)
				return
			}
			//If I get granted a vote
			if response.VoteGranted {
				votedForMe++
				if votedForMe > len(nd.nodos)/2 {
					nd.isLeader = true
					nd.currentLeader = nd.yo
				}
			}
		}
	}

	return

}

//
// PedirVoto RPC method to ask to the other actors a vote to choose a new leader
// ===========
//
// Metodo para RPC PedirVoto
//
func (nd *Nodo) PedirVoto(args *RequestVoteArgs, reply *RequestVoteReply) {
	nd.mux.Lock()
	if nd.currentTerm <= args.Term {
		nd.currentTerm = args.Term
		nd.votedFor = -1
	}

	if nd.votedFor == -1 && nd.currentTerm == args.Term {
		nd.votedFor = args.CandidateID
		nd.currentTerm = args.Term
		reply.VoteGranted = true
		reply.Term = nd.currentTerm
		nd.mux.Unlock()
		return
	}
	reply.VoteGranted = false
	reply.Term = nd.currentTerm
	nd.mux.Unlock()
	return

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
func (nd *Nodo) enviarPeticionVoto(nodo int, args *RequestVoteArgs,
	reply *RequestVoteReply) bool {

	if nd.yo == nodo {
		reply.Term = nd.currentTerm
		reply.VoteGranted = true
		nd.mux.Lock()
		nd.votedFor = nd.yo
		nd.mux.Unlock()
		return true
	}

	ok := nd.nodos[nodo].Call("Nodo.PedirVoto", args, reply)

	return ok != nil && reply.VoteGranted
}

//heartbeatListener is a function that needs to be launched as goroutine. It
//will check if the node is receiving heartbeats from the node
func (nd *Nodo) heartbeatListener() {
	d, err := time.ParseDuration(nd.sleepTimeStr)
	if err != nil {
		os.Exit(1)
	}

	for {
		time.Sleep(d)
		if !nd.isLeader {
			nd.mux.Lock()
			lastHB := nd.lastHeartbeat
			lastHBLimit := nd.lastHeartbeat.Add(d)
			nd.mux.Unlock()
			if lastHB.Before(lastHBLimit) {
				//necesito pedir eleciòn
				nd.EmpezarElecion()
			}
		}
	}
}

func (nd *Nodo) heartbeatListener() {
}
