package tsrpc

import (
	"go/ast"
	"log"
	"reflect"
	"sync/atomic"
)

// service represents an RPC service.
type service struct {
	name   string                 // Name of the service
	typ    reflect.Type           // Type of the service
	rcvr   reflect.Value          // Value of the receiver
	method map[string]*methodType // Map of method names to method types
}

// newService creates a new service instance.
func newService(rcvr interface{}) *service {
	s := new(service)
	s.rcvr = reflect.ValueOf(rcvr)
	s.name = reflect.Indirect(s.rcvr).Type().Name()
	s.typ = reflect.TypeOf(rcvr)
	if !ast.IsExported(s.name) {
		log.Fatalf("rpc server: %s is not a valid service name", s.name)
	}
	s.registerMethods()
	return s
}

// registerMethods registers methods of the service.
func (s *service) registerMethods() {
	s.method = make(map[string]*methodType)
	for i := 0; i < s.typ.NumMethod(); i++ {
		method := s.typ.Method(i)
		mType := method.Type
		if mType.NumIn() != 3 || mType.NumOut() != 1 {
			continue
		}
		if mType.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
			continue
		}
		argType, replyType := mType.In(1), mType.In(2)
		if !isExportedOrBuiltinType(argType) || !isExportedOrBuiltinType(replyType) {
			continue
		}
		s.method[method.Name] = &methodType{
			method:    method,
			ArgType:   argType,
			ReplyType: replyType,
		}
		log.Printf("rpc server: register %s.%s\n", s.name, method.Name)
	}
}

// isExportedOrBuiltinType checks if a type is exported or built-in.
func isExportedOrBuiltinType(t reflect.Type) bool {
	return ast.IsExported(t.Name()) || t.PkgPath() == ""
}

// call invokes a method of the service.
func (s *service) call(m *methodType, argv reflect.Value, replyv reflect.Value) error {
	atomic.AddUint64(&m.numCalls, 1)
	f := m.method.Func
	returnValues := f.Call([]reflect.Value{s.rcvr, argv, replyv})
	if errInter := returnValues[0].Interface(); errInter != nil {
		return errInter.(error)
	}
	return nil
}

// methodType represents a method type.
type methodType struct {
	method    reflect.Method // Method
	ArgType   reflect.Type   // Type of argument
	ReplyType reflect.Type   // Type of reply
	numCalls  uint64         // Number of calls
}

// NumCalls returns the number of calls made to the method.
func (m *methodType) NumCalls() uint64 {
	return atomic.LoadUint64(&m.numCalls)
}

// newArgv creates a new argument value.
func (m *methodType) newArgv() reflect.Value {
	var argv reflect.Value
	if m.ArgType.Kind() == reflect.Ptr {
		argv = reflect.New(m.ArgType.Elem())
	} else {
		argv = reflect.New(m.ArgType).Elem()
	}
	return argv
}

// newReplyv creates a new reply value.
func (m *methodType) newReplyv() reflect.Value {
	replyv := reflect.New(m.ReplyType.Elem())
	switch m.ReplyType.Elem().Kind() {
	case reflect.Map:
		replyv.Elem().Set(reflect.MakeMap(m.ReplyType.Elem()))
	case reflect.Slice:
		replyv.Elem().Set(reflect.MakeSlice(m.ReplyType.Elem(), 0, 0))
	}
	return replyv
}
