package codec

import (
	"bufio"
	"encoding/gob"
	"io"
	"log"
)

// GobCodec implements the Codec interface for encoding and decoding messages using GOB (Go Binary) format.
type GobCodec struct {
	conn io.ReadWriteCloser // conn is the underlying connection.
	buf  *bufio.Writer      // buf is a buffered writer for the connection.
	dec  *gob.Decoder       // dec is a decoder for decoding messages.
	enc  *gob.Encoder       // enc is an encoder for encoding messages.
}

// Ensure GobCodec implements the Codec interface.
var _ Codec = (*GobCodec)(nil)

// NewGobCodec creates a new GobCodec instance with the given connection.
func NewGobCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	return &GobCodec{
		conn: conn,
		buf:  buf,
		dec:  gob.NewDecoder(conn),
		enc:  gob.NewEncoder(buf),
	}
}

// ReadHeader reads the message header.
func (c *GobCodec) ReadHeader(h *Header) error {
	return c.dec.Decode(h)
}

// ReadBody reads the message body into the provided interface.
func (c *GobCodec) ReadBody(body interface{}) error {
	return c.dec.Decode(body)
}

// Write writes the message header and body.
func (c *GobCodec) Write(h *Header, body interface{}) (err error) {
	defer func() {
		_ = c.buf.Flush()
		if err != nil {
			_ = c.Close()
		}
	}()

	// Encode the header.
	if err := c.enc.Encode(h); err != nil {
		return err
	}

	// Encode the body.
	if err := c.enc.Encode(body); err != nil {
		log.Println("rpc codec: gob error encoding body:", err)
		return err
	}
	return nil
}

// Close closes the underlying connection.
func (c *GobCodec) Close() error {
	return c.conn.Close()
}
