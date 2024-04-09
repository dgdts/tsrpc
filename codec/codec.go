package codec

import "io"

// Header represents the metadata associated with each message.
type Header struct {
	ServiceMethod string // ServiceMethod is the name of the service method.
	Seq           uint64 // Seq is the sequence number of the message.
	Error         string // Error contains any error information related to the message.
}

// Codec provides methods for encoding and decoding messages.
type Codec interface {
	io.Closer

	// ReadHeader reads the header of a message.
	ReadHeader(*Header) error

	// ReadBody reads the body of a message into the provided interface.
	ReadBody(interface{}) error

	// Write writes a message header and body.
	Write(*Header, interface{}) error
}

// NewCodecFunc is a function signature for creating a new Codec instance.
type NewCodecFunc func(io.ReadWriteCloser) Codec

// Type represents the type of encoding used by the codec.
type Type string

const (
	// GobType indicates the GOB (Go Binary) encoding type.
	GobType Type = "application/gob"

	// JsonType indicates the JSON encoding type.
	JsonType Type = "application/json"
)

// NewCodecFuncMap maps encoding types to their corresponding NewCodecFunc functions.
var NewCodecFuncMap map[Type]NewCodecFunc

func init() {
	NewCodecFuncMap = make(map[Type]NewCodecFunc)
	// Registering the GOB codec by default
	NewCodecFuncMap[GobType] = NewGobCodec
}
