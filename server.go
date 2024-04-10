// Package tsrpc provides a simple RPC framework.
package tsrpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/dgdts/tsrpc/codec"
)

// MagicNumber is the magic number for identifying TS RPC protocol.
const MagicNumber = 0x3bef5c

// Constants defining default values and paths for TS RPC.
const (
	connected        = "200 Connected to TSRPC"
	defaultRPCPath   = "/_tsrpc_"     // Default path for RPC requests
	defaultDebugPath = "/debug/tsrpc" // Default path for debugging
)

// Option represents the configuration options for the RPC server.
type Option struct {
	MagicNumber    int           // Magic number for protocol identification
	CodecType      codec.Type    // Codec type for serialization and deserialization
	ConnectTimeout time.Duration // Connection timeout duration
	HandleTimeout  time.Duration // Request handling timeout duration
}

// DefaultOption provides default configuration options.
var DefaultOption = &Option{
	MagicNumber:    MagicNumber,
	CodecType:      codec.GobType,
	ConnectTimeout: 0,
}

// Server represents an RPC server.
type Server struct {
	serviceMap  sync.Map      // Map of service names to services
	callTimeout time.Duration // Timeout duration for method calls

	readLock sync.Mutex // Mutex for reading requests
}

// Register registers a service with the RPC server.
func (s *Server) Register(rcvr interface{}) error {
	service := newService(rcvr)
	if _, dup := s.serviceMap.LoadOrStore(service.name, service); dup {
		return errors.New("rpc: service already defined: " + service.name)
	}
	return nil
}

// Register registers a service with the default RPC server.
func Register(rcvr interface{}) error {
	return DefaultServer.Register(rcvr)
}

// findService finds the service and method type corresponding to a given service method name.
func (s *Server) findService(serviceMethod string) (svc *service, mtype *methodType, err error) {
	dot := strings.LastIndex(serviceMethod, ".")
	if dot < 0 {
		err = errors.New("rpc server: service/method request ill-formed:" + serviceMethod)
		return nil, nil, err
	}
	serviceName, methodName := serviceMethod[:dot], serviceMethod[dot+1:]
	svci, ok := s.serviceMap.Load(serviceName)
	if !ok {
		err = errors.New("rpc server: can't find service:" + serviceName)
		return
	}
	svc = svci.(*service)
	mtype = svc.method[methodName]
	if mtype == nil {
		err = errors.New("rpc server: can't find method:" + methodName)
	}
	return
}

// NewServer creates a new instance of the RPC server.
func NewServer() *Server {
	return &Server{}
}

// DefaultServer represents the default RPC server.
var DefaultServer = NewServer()

// Accept accepts incoming connections on the provided listener and serves them.
func (s *Server) Accept(lis net.Listener) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Println("rpc server: accept error:", err)
			return
		}
		go s.ServeConn(conn)
	}
}

// Accept accepts incoming connections on the provided listener and serves them using the default server.
func Accept(lis net.Listener) {
	DefaultServer.Accept(lis)
}

// ServeConn serves RPC requests on a single connection.
func (s *Server) ServeConn(conn io.ReadWriteCloser) {
	defer func() {
		_ = conn.Close()
	}()

	var opt Option
	if err := json.NewDecoder(conn).Decode(&opt); err != nil {
		log.Println("rpc server: options error:", err)
		return
	}

	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		log.Printf("rpc server: invalid codec type %s", opt.CodecType)
		return
	}
	cc := f(conn)

	if err := json.NewEncoder(conn).Encode(opt); err != nil {
		log.Println("rpc server: option error:", err)
	}

	s.serveCodec(cc)
}

// invalidRequest is a placeholder for invalid requests.
var invalidRequest = struct{}{}

// serveCodec serves requests using the specified codec.
func (s *Server) serveCodec(cc codec.Codec) {
	sending := new(sync.Mutex)
	wg := new(sync.WaitGroup)
	for {
		req, err := s.readRequest(cc)
		if err != nil {
			if req == nil {
				break
			}
			req.h.Error = err.Error()
			s.sendResponse(cc, req.h, invalidRequest, sending)
			continue
		}
		wg.Add(1)
		go s.handleRequest(cc, req, sending, wg, s.callTimeout)
	}
	wg.Wait()
	_ = cc.Close()
}

// request represents an RPC request.
type request struct {
	h      *codec.Header // Request header
	argv   reflect.Value // Argument value
	replyv reflect.Value // Reply value
	mtype  *methodType   // Method type
	svc    *service      // Service
}

// readRequestHeader reads the header of an RPC request.
func (s *Server) readRequestHeader(cc codec.Codec) (*codec.Header, error) {
	s.readLock.Lock()
	defer s.readLock.Unlock()

	var h codec.Header
	if err := cc.ReadHeader(&h); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			log.Println("rpc server: read header error:", err)
		}
		return nil, err
	}
	return &h, nil
}

// readRequest reads an RPC request.
func (s *Server) readRequest(cc codec.Codec) (*request, error) {
	h, err := s.readRequestHeader(cc)
	if err != nil {
		return nil, err
	}
	req := &request{h: h}

	req.svc, req.mtype, err = s.findService(h.ServiceMethod)
	if err != nil {
		return req, err
	}
	req.argv = req.mtype.newArgv()
	req.replyv = req.mtype.newReplyv()

	argvi := req.argv.Interface()
	if req.argv.Type().Kind() != reflect.Ptr {
		argvi = req.argv.Addr().Interface()
	}

	if err = cc.ReadBody(argvi); err != nil {
		log.Println("rpc server: read argv err:", err)
	}
	return req, nil
}

// sendResponse sends an RPC response.
func (s *Server) sendResponse(cc codec.Codec, h *codec.Header, body interface{}, sending *sync.Mutex) {
	sending.Lock()
	defer sending.Unlock()
	if err := cc.Write(h, body); err != nil {
		log.Println("rpc server: write response error:", err)
	}
}

// handleRequest handles an RPC request.
func (s *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup, timeout time.Duration) {
	defer wg.Done()
	called := make(chan struct{})
	sent := make(chan struct{})
	go func() {
		err := req.svc.call(req.mtype, req.argv, req.replyv)
		called <- struct{}{}
		if err != nil {
			req.h.Error = err.Error()
			s.sendResponse(cc, req.h, invalidRequest, sending)
			sent <- struct{}{}
			return
		}
		s.sendResponse(cc, req.h, req.replyv.Interface(), sending)
		sent <- struct{}{}
	}()
	if timeout == 0 {
		<-called
		<-sent
		return
	}
	select {
	case <-time.After(timeout):
		req.h.Error = fmt.Sprintf("rpc server: request handle timeout: expect within %s", timeout)
		s.sendResponse(cc, req.h, req.replyv.Interface(), sending)
	case <-called:
		<-sent
	}
}

// ServeHTTP serves HTTP requests.
func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != "CONNECT" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = io.WriteString(w, "405 must CONNEST\n")
		return
	}
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		log.Print("rpc hijacking", req.RemoteAddr, ": ", err.Error())
		return
	}
	_, _ = io.WriteString(conn, "HTTP/1.0 "+connected+"\n\n")
	s.ServeConn(conn)
}

// HandleHTTP registers the HTTP handlers for RPC requests and debugging.
func (s *Server) HandleHTTP() {
	http.Handle(defaultRPCPath, s)
	http.Handle(defaultDebugPath, debugHTTP{s})
	log.Println("rpc server debug path:", defaultDebugPath)
}

// HandleHTTP registers the HTTP handlers for RPC requests and debugging using the default server.
func HandleHTTP() {
	DefaultServer.HandleHTTP()
}
