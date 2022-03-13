package tcp

// type tcpTransportClient[T xtransport.Packet] tcpSocket[T]

// type tcpTransportClient[T xtransport.Packet] struct {
// 	dialOpts xtransport.DialOptions
// 	conn     net.Conn
// 	encBuf   *bufio.Writer
// 	*xtransport.Context
// 	timeout time.Duration
// }

// func (t *tcpTransportClient[T]) Local() string {
// 	return t.conn.LocalAddr().String()
// }

// func (t *tcpTransportClient[T]) Remote() string {
// 	return t.conn.RemoteAddr().String()
// }

// func (t *tcpTransportClient[T]) ConnectionState() *tls.ConnectionState {
// 	if c2, ok := t.conn.(*tls.Conn); ok {
// 		tmp := c2.ConnectionState()
// 		return &tmp
// 	}
// 	return nil
// }

// func (t *tcpTransportClient[T]) SetTimeOut(duration time.Duration) {
// 	t.timeout = duration
// }

// func (t *tcpTransportClient[T]) Send(m T) error {
// 	// set timeout if its greater than 0
// 	if t.timeout > time.Duration(0) {
// 		t.conn.SetDeadline(time.Now().Add(t.timeout))
// 	}
// 	// gob.NewEncoder()
// 	if err := m.Write(t.encBuf); err != nil {
// 		return err
// 	}
// 	return t.encBuf.Flush()
// }

// func (t *tcpTransportClient[T]) Recv(fc func(r io.Reader) (T, error)) (T, error) {
// 	// set timeout if its greater than 0
// 	if t.timeout > time.Duration(0) {
// 		t.conn.SetDeadline(time.Now().Add(t.timeout))
// 	}
// 	return fc(t.conn)
// }

// func (t *tcpTransportClient[T]) Close() error {
// 	return t.conn.Close()
// }
