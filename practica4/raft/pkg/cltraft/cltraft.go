package cltraft

/*Functions that simulate a client that send Raft a request.

 */

import (
	"fmt"
	"net/rpc"
	"time"
)

//CallTimeout executes an RPC call to the client, returning error if there was a timeout
//(no response) or nil if the call was successful
func CallTimeout(client *rpc.Client, serviceMethod string, args interface{},
	reply interface{}, timeout time.Duration) error {
	done := client.Go(serviceMethod, args, reply, make(chan *rpc.Call, 1)).Done

	select {
	case call := <-done:
		return call.Error
	case <-time.After(timeout):
		return fmt.Errorf(
			"timeout in CallTimeout with method: %s, args: %v",
			serviceMethod,
			args)
	}
}
