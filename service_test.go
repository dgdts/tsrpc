package tsrpc

import (
	"fmt"
	"reflect"
	"testing"
)

// Foo represents a sample type for testing.
type Foo int

// Args represents arguments for testing.
type Args struct{ Num1, Num2 int }

// Sum is a method of Foo that sums two integers.
func (f Foo) Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

// sum is a method of Foo (lowercase) that sums two integers.
// should not be export.
func (f Foo) sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

// _assert is a helper function for testing assertions.
func _assert(condition bool, msg string, v ...interface{}) {
	if !condition {
		panic(fmt.Sprintf("assertion failed: "+msg, v...))
	}
}

// TestNewService tests the NewService function.
func TestNewService(t *testing.T) {
	var foo Foo
	s := newService(&foo)
	_assert(len(s.method) == 1, "wrong service Method, expect 1, but got %d", len(s.method))
	mType := s.method["Sum"]
	_assert(mType != nil, "wrong Method, Sum should't nil")
}

// TestMethodType_Call tests the Call method of MethodType.
func TestMethodType_Call(t *testing.T) {
	var foo Foo
	s := newService(&foo)
	mType := s.method["Sum"]

	argv := mType.newArgv()
	replyv := mType.newReplyv()
	argv.Set(reflect.ValueOf(Args{Num1: 1, Num2: 2}))
	err := s.call(mType, argv, replyv)
	_assert(err == nil && *replyv.Interface().(*int) == 3 && mType.numCalls == 1, "failed to call Foo.Sum")
}
