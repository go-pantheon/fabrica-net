package wsconn

import (
	"io"
	"net"
	"time"

	"github.com/go-pantheon/fabrica-util/errors"
	"github.com/gorilla/websocket"
)

var ErrInvalidFrameType = errors.New("invalid frame type")

const (
	MessageType = websocket.BinaryMessage
)

var _ net.Conn = (*WebSocketConn)(nil)

type WebSocketConn struct {
	conn *websocket.Conn
}

func NewWebSocketConn(conn *websocket.Conn) (c *WebSocketConn) {
	return &WebSocketConn{
		conn: conn,
	}
}

// Read implements net.Conn interface
func (c *WebSocketConn) Read(b []byte) (n int, err error) {
	mt, r, err := c.conn.NextReader()
	if err != nil {
		return 0, err
	}

	if mt != MessageType {
		return 0, ErrInvalidFrameType
	}

	return r.Read(b)
}

// Write implements net.Conn interface
func (c *WebSocketConn) Write(b []byte) (n int, err error) {
	w, err := c.conn.NextWriter(MessageType)
	if err != nil {
		return 0, err
	}

	defer func() {
		if closeErr := w.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	return w.Write(b)
}

// Close implements net.Conn interface
func (c *WebSocketConn) Close() (err error) {
	msg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "conn closed")

	if writeErr := c.conn.WriteMessage(websocket.CloseMessage, msg); writeErr != nil {
		err = errors.Join(err, errors.Wrap(writeErr, "write close message failed"))
	}

	time.Sleep(1 * time.Millisecond)

	if closeErr := c.conn.Close(); closeErr != nil {
		err = errors.Join(err, errors.Wrap(closeErr, "close connection failed"))
	}

	return err
}

// LocalAddr implements net.Conn interface
func (c *WebSocketConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// RemoteAddr implements net.Conn interface
func (c *WebSocketConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// SetDeadline implements net.Conn interface
func (c *WebSocketConn) SetDeadline(t time.Time) error {
	if err := c.conn.SetReadDeadline(t); err != nil {
		return err
	}

	return c.conn.SetWriteDeadline(t)
}

// SetReadDeadline implements net.Conn interface
func (c *WebSocketConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

// SetWriteDeadline implements net.Conn interface
func (c *WebSocketConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

func (c *WebSocketConn) NextWriter(mt int) (io.WriteCloser, error) {
	return c.conn.NextWriter(mt)
}

func (c *WebSocketConn) NextReader() (int, io.Reader, error) {
	return c.conn.NextReader()
}

func (c *WebSocketConn) SetPongHandler(f func(string) error) {
	c.conn.SetPongHandler(f)
}

func (c *WebSocketConn) SetReadLimit(limit int64) {
	c.conn.SetReadLimit(limit)
}
