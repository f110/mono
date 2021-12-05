package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	merrors "go.f110.dev/go-memcached/errors"
)

var (
	crlf                 = []byte("\r\n")
	msgTextProtoStored   = []byte("STORED\r\n")
	msgTextProtoDeleted  = []byte("DELETED\r\n")
	msgTextProtoNotFound = []byte("NOT_FOUND\r\n")
	msgTextProtoTouched  = []byte("TOUCHED\r\n")
	msgTextProtoExists   = []byte("EXISTS\r\n")
	msgTextProtoOk       = []byte("OK\r\n")
	msgTextProtoEnd      = []byte("END\r\n")

	valueFormatWithCas = "VALUE %s %d %d %d"
	valueFormat        = "VALUE %s %d %d"
)

var (
	msgMetaProtoEnd       = []byte("EN\r\n")
	msgMetaProtoNoop      = []byte("MN\r\n")
	msgMetaProtoStored    = []byte("HD")
	msgMetaProtoNotStored = []byte("NS")
	msgMetaProtoDeleted   = []byte("HD")
	msgMetaProtoExists    = []byte("EX")
	msgMetaProtoNotFound  = []byte("NF")
)

var (
	magicRequest  byte = 0x80
	magicResponse byte = 0x81
)

const (
	binaryOpcodeGet byte = iota
	binaryOpcodeSet
	binaryOpcodeAdd
	binaryOpcodeReplace
	binaryOpcodeDelete
	binaryOpcodeIncrement
	binaryOpcodeDecrement
	binaryOpcodeQuit
	binaryOpcodeFlush
	binaryOpcodeGetQ
	binaryOpcodeNoop
	binaryOpcodeVersion
	binaryOpcodeGetK
	binaryOpcodeGetKQ
	binaryOpcodeAppend
	binaryOpcodePrepend
	binaryOpcodeTouch byte = 28 // 0x1c
)

var binaryStatus = map[uint16]string{
	1:   "key not found",
	2:   "key exists",
	3:   "value too large",
	4:   "invalid arguments",
	6:   "non-numeric value",
	129: "unknown command",
	130: "out of memory",
}

type ServerOperationType string

const (
	ServerDeleteOnly ServerOperationType = "delete_only"
	ServerWriteOnly  ServerOperationType = "write_only"
)

type Server interface {
	Name() string
	Close() error
	Get(key string) (*Item, error)
	GetMulti(keys ...string) ([]*Item, error)
	Set(item *Item) error
	Add(item *Item) error
	Replace(item *Item) error
	Delete(key string) error
	Increment(key string, delta int, expiration int) (int64, error)
	Decrement(key string, delta int, expiration int) (int64, error)
	Touch(key string, expiration int) error
	Flush() error
	Version() (string, error)
}

type ServerWithTextProtocol struct {
	name    string
	Network string
	Addr    string
	Type    ServerOperationType

	conn *bufio.ReadWriter
	raw  net.Conn
	mu   sync.Mutex
}

var _ Server = &ServerWithTextProtocol{}

func NewServerWithTextProtocol(ctx context.Context, name, network, addr string) (*ServerWithTextProtocol, error) {
	d := &net.Dialer{}
	conn, err := d.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	return &ServerWithTextProtocol{
		name:    name,
		Network: network,
		Addr:    addr,
		conn:    bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn)),
		raw:     conn,
	}, nil
}

func (s *ServerWithTextProtocol) Name() string {
	return s.name
}

func (s *ServerWithTextProtocol) Close() error {
	return s.raw.Close()
}

func (s *ServerWithTextProtocol) Get(key string) (*Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := fmt.Fprintf(s.conn, "gets %s\r\n", key); err != nil {
		return nil, err
	}
	if err := s.conn.Flush(); err != nil {
		return nil, err
	}
	b, err := s.conn.ReadSlice('\n')
	if err != nil {
		return nil, err
	}
	if bytes.Equal(b, msgTextProtoEnd) {
		return nil, merrors.ItemNotFound
	}
	if !bytes.HasPrefix(b, []byte("VALUE")) {
		return nil, errors.New("memcached: invalid response")
	}
	item := &Item{Server: s.Name()}
	size := 0
	p := valueFormatWithCas
	var flags uint32
	scan := []interface{}{&item.Key, &flags, &size, &item.Cas}
	if bytes.Count(b, []byte(" ")) == 3 {
		p = valueFormat
		scan = scan[:3]
	}
	if _, err := fmt.Fscanf(bytes.NewReader(b), p, scan...); err != nil {
		return nil, err
	}
	f := make([]byte, 4)
	binary.BigEndian.PutUint32(f, flags)
	item.Flags = f

	buf := make([]byte, size+2)
	if _, err := io.ReadFull(s.conn, buf); err != nil {
		return nil, err
	}
	item.Value = buf[:size]

	b, err = s.conn.ReadSlice('\n')
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(b, msgTextProtoEnd) {
		return nil, errors.New("memcached: invalid response")
	}

	return item, nil
}

func (s *ServerWithTextProtocol) GetMulti(keys ...string) ([]*Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	items := make([]*Item, 0, len(keys))
	if _, err := fmt.Fprintf(s.conn, "gets %s\r\n", strings.Join(keys, " ")); err != nil {
		return nil, err
	}
	if err := s.conn.Flush(); err != nil {
		return nil, err
	}
	i, err := s.parseGetResponse(s.conn.Reader)
	if err != nil {
		return nil, err
	}
	for _, v := range i {
		v.Server = s.Name()
	}
	items = append(items, i...)

	return items, nil
}

func (s *ServerWithTextProtocol) parseGetResponse(r *bufio.Reader) ([]*Item, error) {
	res := make([]*Item, 0)
	for {
		b, err := r.ReadSlice('\n')
		if err != nil {
			return nil, err
		}
		if bytes.Equal(b, []byte("END\r\n")) {
			break
		}
		s := bytes.Split(b, []byte(" "))
		if !bytes.Equal(s[0], []byte("VALUE")) {
			return nil, errors.New("memcached: invalid response")
		}

		f, err := strconv.Atoi(string(s[2]))
		if err != nil {
			return nil, err
		}
		flags := make([]byte, 4)
		binary.BigEndian.PutUint32(flags, uint32(f))

		var size int
		if len(s) == 4 {
			size, err = strconv.Atoi(string(s[3][:len(s[3])-2]))
		} else {
			size, err = strconv.Atoi(string(s[3]))
		}
		if err != nil {
			return nil, err
		}

		buf := make([]byte, size+2)
		item := &Item{
			Key:   string(s[1]),
			Flags: flags,
		}
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		item.Value = buf[:size]
		res = append(res, item)
	}

	return res, nil
}

func (s *ServerWithTextProtocol) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := fmt.Fprintf(s.conn, "delete %s\r\n", key); err != nil {
		return err
	}
	if err := s.conn.Flush(); err != nil {
		return err
	}
	b, err := s.conn.ReadSlice('\n')
	if err != nil {
		return err
	}

	switch {
	case bytes.Equal(b, msgTextProtoDeleted):
		return nil
	case bytes.Equal(b, msgTextProtoNotFound):
		return merrors.ItemNotFound
	}
	return nil
}

func (s *ServerWithTextProtocol) Increment(key string, delta, _ int) (int64, error) {
	return s.incrOrDecr("incr", key, delta)
}

func (s *ServerWithTextProtocol) Decrement(key string, delta, _ int) (int64, error) {
	return s.incrOrDecr("decr", key, delta)
}

func (s *ServerWithTextProtocol) incrOrDecr(op, key string, delta int) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := fmt.Fprintf(s.conn, "%s %s %d\r\n", op, key, delta); err != nil {
		return 0, err
	}
	if err := s.conn.Flush(); err != nil {
		return 0, err
	}
	b, err := s.conn.ReadSlice('\n')
	if err != nil {
		return 0, err
	}

	switch {
	case bytes.Equal(b, msgTextProtoNotFound):
		return 0, merrors.ItemNotFound
	default:
		i, err := strconv.ParseInt(string(b[:len(b)-2]), 10, 64)
		if err != nil {
			return 0, err
		}
		return i, nil
	}
}

func (s *ServerWithTextProtocol) Touch(key string, expiration int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := fmt.Fprintf(s.conn, "touch %s %d\r\n", key, expiration); err != nil {
		return err
	}
	if err := s.conn.Flush(); err != nil {
		return err
	}
	b, err := s.conn.ReadSlice('\n')
	if err != nil {
		return err
	}

	switch {
	case bytes.Equal(b, msgTextProtoTouched):
		return nil
	case bytes.Equal(b, msgTextProtoNotFound):
		return merrors.ItemNotFound
	}

	return nil
}

func (s *ServerWithTextProtocol) Set(item *Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	flags := uint32(0)
	if len(item.Flags) == 4 {
		flags = binary.BigEndian.Uint32(item.Flags)
	}
	if item.Cas > 0 {
		if _, err := fmt.Fprintf(s.conn, "cas %s %d %d %d %d\r\n", item.Key, flags, item.Expiration, len(item.Value), item.Cas); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(s.conn, "set %s %d %d %d\r\n", item.Key, flags, item.Expiration, len(item.Value)); err != nil {
			return err
		}
	}
	if _, err := s.conn.Write(append(item.Value, crlf...)); err != nil {
		return err
	}
	if err := s.conn.Flush(); err != nil {
		return err
	}
	buf, err := s.conn.ReadSlice('\n')
	if err != nil {
		return err
	}

	switch {
	case bytes.Equal(buf, msgTextProtoStored):
		return nil
	case bytes.Equal(buf, msgTextProtoExists):
		return merrors.ItemExists
	default:
		return fmt.Errorf("memcached: failed set: %s", string(buf))
	}
}

func (s *ServerWithTextProtocol) Add(item *Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	flags := uint32(0)
	if len(item.Flags) == 4 {
		flags = binary.BigEndian.Uint32(item.Flags)
	}
	if _, err := fmt.Fprintf(s.conn, "add %s %d %d %d\r\n", item.Key, flags, item.Expiration, len(item.Value)); err != nil {
		return err
	}
	s.conn.Write(item.Value)
	s.conn.Write(crlf)
	if err := s.conn.Flush(); err != nil {
		return err
	}

	b, err := s.conn.ReadSlice('\n')
	if err != nil {
		return err
	}
	if bytes.Equal(b, msgTextProtoStored) {
		return nil
	}

	return merrors.ItemExists
}

func (s *ServerWithTextProtocol) Replace(item *Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	flags := uint32(0)
	if len(item.Flags) == 4 {
		flags = binary.BigEndian.Uint32(item.Flags)
	}
	if _, err := fmt.Fprintf(s.conn, "replace %s %d %d %d\r\n", item.Key, flags, item.Expiration, len(item.Value)); err != nil {
		return err
	}
	s.conn.Write(item.Value)
	s.conn.Write(crlf)
	if err := s.conn.Flush(); err != nil {
		return err
	}

	b, err := s.conn.ReadSlice('\n')
	if err != nil {
		return err
	}
	if bytes.Equal(b, msgTextProtoStored) {
		return nil
	}

	return merrors.ItemNotFound
}

func (s *ServerWithTextProtocol) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := fmt.Fprint(s.conn, "flush_all\r\n"); err != nil {
		return err
	}
	if err := s.conn.Flush(); err != nil {
		return err
	}
	b, err := s.conn.ReadSlice('\n')
	if err != nil {
		return err
	}

	switch {
	case bytes.Equal(b, msgTextProtoOk):
		return nil
	default:
		return fmt.Errorf("memcached: %s", string(b[:len(b)-2]))
	}
}

func (s *ServerWithTextProtocol) Version() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.conn.WriteString("version\r\n"); err != nil {
		return "", err
	}
	if err := s.conn.Flush(); err != nil {
		return "", err
	}

	b, err := s.conn.ReadSlice('\n')
	if err != nil {
		return "", err
	}

	if len(b) > 0 {
		return strings.TrimSuffix(strings.TrimPrefix(string(b), "VERSION "), "\r\n"), nil
	}
	return "", err
}

type ServerWithMetaProtocol struct {
	name    string
	Network string
	Addr    string
	Type    ServerOperationType

	conn *bufio.ReadWriter
	raw  net.Conn
	mu   sync.Mutex
}

var _ Server = &ServerWithMetaProtocol{}

func NewServerWithMetaProtocol(ctx context.Context, name, network, addr string) (*ServerWithMetaProtocol, error) {
	d := &net.Dialer{}
	conn, err := d.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	return &ServerWithMetaProtocol{
		name:    name,
		Network: network,
		Addr:    addr,
		conn:    bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn)),
		raw:     conn,
	}, nil
}

func (s *ServerWithMetaProtocol) Name() string {
	return s.name
}

func (s *ServerWithMetaProtocol) Get(key string) (*Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := fmt.Fprintf(s.conn, "mg %s v f c t\r\n", key); err != nil {
		return nil, err
	}
	if err := s.conn.Flush(); err != nil {
		return nil, err
	}

	item, err := s.parseGetResponse(s.conn)
	if err != nil {
		return nil, err
	}
	item.Key = key
	return item, nil
}

func (s *ServerWithMetaProtocol) GetMulti(keys ...string) ([]*Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	items := make([]*Item, 0)
	for _, key := range keys {
		if _, err := fmt.Fprintf(s.conn, "mg %s v f k c t\r\n", key); err != nil {
			return nil, err
		}
	}
	s.conn.WriteString("mn\r\n")
	if err := s.conn.Flush(); err != nil {
		return nil, err
	}

MultiRead:
	for {
		item, err := s.parseGetResponse(s.conn)
		if err != nil {
			switch err {
			case merrors.ItemNotFound:
				break MultiRead
			default:
				return nil, err
			}
		}
		items = append(items, item)
	}

	return items, nil
}

func (s *ServerWithMetaProtocol) parseGetResponse(conn *bufio.ReadWriter) (*Item, error) {
	b, err := conn.ReadSlice('\n')
	if err != nil {
		return nil, err
	}
	if bytes.Equal(b, msgMetaProtoEnd) {
		return nil, merrors.ItemNotFound
	}
	if bytes.Equal(b, msgMetaProtoNoop) {
		return nil, merrors.ItemNotFound
	}
	if !bytes.HasPrefix(b, []byte("VA")) {
		return nil, errors.New("memcached: invalid get response")
	}

	item := &Item{}
	size := 0
	// state:
	//    1 - Reading size
	//    2 - Reading flag name
	//    3 - Reading token
	state := 1 // Read size
	var flagName byte
	var startIndex int
	for i := range b[3:] {
		switch b[3+i] {
		case ' ':
			switch state {
			case 1: // Reading size
				s, err := strconv.ParseInt(string(b[3:3+i]), 10, 32)
				if err != nil {
					return nil, err
				}
				size = int(s)
				state = 2
			case 3: // Reading token
				if err := s.readGetFlagToken(flagName, b[startIndex:3+i], item); err != nil {
					return nil, err
				}
				state = 2
			}
		case '\r':
			if state == 3 {
				if err := s.readGetFlagToken(flagName, b[startIndex:3+i], item); err != nil {
					return nil, err
				}
			}
		case 'f', 'c', 't', 'k':
			switch state {
			case 2:
				flagName = b[3+i]
				state = 3
				startIndex = 3 + i + 1
			}
		}
	}

	buf := make([]byte, size+2)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return nil, err
	}
	item.Value = buf[:size]

	return item, nil
}

func (s *ServerWithMetaProtocol) readGetFlagToken(flagName byte, v []byte, item *Item) error {
	switch flagName {
	case 'f':
		in, err := strconv.ParseInt(string(v), 10, 32)
		if err != nil {
			return err
		}
		f := make([]byte, 4)
		binary.BigEndian.PutUint32(f, uint32(in))
		item.Flags = f
	case 'c':
		in, err := strconv.ParseInt(string(v), 10, 64)
		if err != nil {
			return err
		}
		item.Cas = uint64(in)
	case 't':
		in, err := strconv.ParseInt(string(v), 10, 32)
		if err != nil {
			return err
		}
		item.Expiration = int(in)
	case 'k':
		item.Key = string(v)
	}

	return nil
}

func (s *ServerWithMetaProtocol) Set(item *Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	format := "%s %d T%d" // ms <key> <datalen> TTL
	values := []interface{}{item.Key, len(item.Value), item.Expiration}
	if item.Cas > 0 {
		format += " C%d"
		values = append(values, item.Cas)
	}
	if item.Flags != nil {
		format += " F%d"
		values = append(values, binary.BigEndian.Uint32(item.Flags))
	}
	if _, err := fmt.Fprintf(s.conn, "ms "+format+"\r\n", values...); err != nil {
		return err
	}
	if _, err := s.conn.Write(append(item.Value, crlf...)); err != nil {
		return err
	}
	if err := s.conn.Flush(); err != nil {
		return err
	}
	b, err := s.conn.ReadSlice('\n')
	if err != nil {
		return err
	}
	switch {
	case bytes.HasPrefix(b, msgMetaProtoStored):
		return nil
	case bytes.HasPrefix(b, msgMetaProtoNotStored):
		return errors.New("memcached: not stored")
	case bytes.HasPrefix(b, msgMetaProtoExists):
		return merrors.ItemExists
	case bytes.HasPrefix(b, msgMetaProtoNotFound):
		return merrors.ItemNotFound
	}

	log.Print(string(b))
	return errors.New("memcached: failed set")
}

func (s *ServerWithMetaProtocol) Add(item *Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// FIXME: should use new text protocol
	flags := uint32(0)
	if len(item.Flags) == 4 {
		flags = binary.BigEndian.Uint32(item.Flags)
	}
	if _, err := fmt.Fprintf(s.conn, "add %s %d %d %d\r\n", item.Key, flags, item.Expiration, len(item.Value)); err != nil {
		return err
	}
	s.conn.Write(item.Value)
	s.conn.Write(crlf)
	if err := s.conn.Flush(); err != nil {
		return err
	}

	b, err := s.conn.ReadSlice('\n')
	if err != nil {
		return err
	}
	if bytes.Equal(b, msgTextProtoStored) {
		return nil
	}

	return merrors.ItemExists
}

func (s *ServerWithMetaProtocol) Replace(item *Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// FIXME: should use new text protocol
	flags := uint32(0)
	if len(item.Flags) == 4 {
		flags = binary.BigEndian.Uint32(item.Flags)
	}
	if _, err := fmt.Fprintf(s.conn, "replace %s %d %d %d\r\n", item.Key, flags, item.Expiration, len(item.Value)); err != nil {
		return err
	}
	s.conn.Write(item.Value)
	s.conn.Write(crlf)
	if err := s.conn.Flush(); err != nil {
		return err
	}

	b, err := s.conn.ReadSlice('\n')
	if err != nil {
		return err
	}
	if bytes.Equal(b, msgTextProtoStored) {
		return nil
	}

	return merrors.ItemNotFound
}

func (s *ServerWithMetaProtocol) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := fmt.Fprintf(s.conn, "md %s\r\n", key); err != nil {
		return err
	}
	if err := s.conn.Flush(); err != nil {
		return err
	}
	b, err := s.conn.ReadSlice('\n')
	if err != nil {
		return err
	}

	switch {
	case bytes.HasPrefix(b, msgMetaProtoDeleted):
		return nil
	case bytes.HasPrefix(b, msgMetaProtoNotFound):
		return nil
	case bytes.HasPrefix(b, msgMetaProtoExists):
		return merrors.ItemNotFound
	default:
		return errors.New("memcached: failed delete")
	}
}

// Increment is increment value when if exist key.
// implement is same as textProtocol.Incr.
func (s *ServerWithMetaProtocol) Increment(key string, delta, _ int) (int64, error) {
	return s.incrOrDecr("incr", key, delta)
}

// Decrement is decrement value when if exist key.
// implement is same as textProtocol.Decr.
func (s *ServerWithMetaProtocol) Decrement(key string, delta, _ int) (int64, error) {
	return s.incrOrDecr("decr", key, delta)
}

func (s *ServerWithMetaProtocol) incrOrDecr(op, key string, delta int) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := fmt.Fprintf(s.conn, "%s %s %d\r\n", op, key, delta); err != nil {
		return 0, err
	}
	if err := s.conn.Flush(); err != nil {
		return 0, err
	}
	b, err := s.conn.ReadSlice('\n')
	if err != nil {
		return 0, err
	}

	switch {
	case bytes.Equal(b, msgTextProtoNotFound):
		return 0, merrors.ItemNotFound
	default:
		i, err := strconv.ParseInt(string(b[:len(b)-2]), 10, 64)
		if err != nil {
			return 0, err
		}
		return i, nil
	}
}

func (s *ServerWithMetaProtocol) Touch(key string, expiration int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := fmt.Fprintf(s.conn, "md %s I T%d\r\n", key, expiration); err != nil {
		return err
	}
	if err := s.conn.Flush(); err != nil {
		return err
	}

	b, err := s.conn.ReadSlice('\n')
	if err != nil {
		return err
	}

	switch {
	case bytes.HasPrefix(b, msgMetaProtoDeleted):
		return nil
	default:
		return errors.New("memcached: failed touch")
	}
}

func (s *ServerWithMetaProtocol) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := fmt.Fprint(s.conn, "flush_all\r\n"); err != nil {
		return err
	}
	if err := s.conn.Flush(); err != nil {
		return err
	}
	b, err := s.conn.ReadSlice('\n')
	if err != nil {
		return err
	}

	switch {
	case bytes.Equal(b, msgTextProtoOk):
		return nil
	default:
		return fmt.Errorf("memcached: %s", string(b[:len(b)-2]))
	}
}

func (s *ServerWithMetaProtocol) Version() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.conn.WriteString("version\r\n"); err != nil {
		return "", err
	}
	if err := s.conn.Flush(); err != nil {
		return "", err
	}

	b, err := s.conn.ReadSlice('\n')
	if err != nil {
		return "", err
	}
	if len(b) > 0 {
		return strings.TrimSuffix(strings.TrimPrefix(string(b), "VERSION "), "\r\n"), nil
	}

	return "", nil
}

func (s *ServerWithMetaProtocol) Close() error {
	return s.raw.Close()
}

type ServerWithBinaryProtocol struct {
	name    string
	Network string
	Addr    string
	Type    ServerOperationType
	Timeout time.Duration

	mu   sync.Mutex
	conn *bufio.ReadWriter
	raw  net.Conn

	reqHeaderPool *sync.Pool
	resHeaderPool *sync.Pool
}

var _ Server = &ServerWithBinaryProtocol{}

func NewServerWithBinaryProtocol(ctx context.Context, name, network, addr string) (*ServerWithBinaryProtocol, error) {
	d := &net.Dialer{}
	conn, err := d.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	return &ServerWithBinaryProtocol{
		name:    name,
		Network: network,
		Addr:    addr,
		conn:    bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn)),
		raw:     conn,
		reqHeaderPool: &sync.Pool{New: func() interface{} {
			return newBinaryRequestHeader()
		}},
		resHeaderPool: &sync.Pool{New: func() interface{} {
			return newBinaryResponseHeader()
		}},
	}, nil
}

func (s *ServerWithBinaryProtocol) Name() string {
	return s.name
}

func (s *ServerWithBinaryProtocol) Get(key string) (*Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Timeout != 0 {
		s.raw.SetWriteDeadline(time.Now().Add(s.Timeout))
		s.raw.SetReadDeadline(time.Now().Add(s.Timeout))
	}

	header := s.reqHeaderPool.Get().(*binaryRequestHeader)
	defer s.reqHeaderPool.Put(header)
	header.Reset()

	header.Opcode = binaryOpcodeGet
	header.KeyLength = uint16(len(key))
	header.TotalBodyLength = uint32(len(key))
	if err := header.EncodeTo(s.conn); err != nil {
		return nil, err
	}
	s.conn.Write([]byte(key))
	if err := s.conn.Flush(); err != nil {
		return nil, err
	}

	resHeader := s.resHeaderPool.Get().(*binaryResponseHeader)
	defer s.resHeaderPool.Put(resHeader)

	if err := resHeader.Read(s.conn); err != nil {
		return nil, err
	}

	buf := make([]byte, resHeader.TotalBodyLength)
	if _, err := io.ReadFull(s.conn, buf); err != nil {
		return nil, err
	}

	if resHeader.Status != 0 {
		if v, ok := binaryStatus[resHeader.Status]; ok {
			switch resHeader.Status {
			case 1: // key not found
				return nil, merrors.ItemNotFound
			default:
				return nil, fmt.Errorf("memcached: error %s", v)
			}
		}
		return nil, fmt.Errorf("memcached: unknown error %d", resHeader.Status)
	}

	return &Item{
		Key:   key,
		Value: buf[uint16(resHeader.ExtraLength)+resHeader.KeyLength:],
		Flags: buf[:resHeader.ExtraLength],
		Cas:   resHeader.CAS,
	}, nil
}

func (s *ServerWithBinaryProtocol) GetMulti(keys ...string) ([]*Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Timeout != 0 {
		s.raw.SetWriteDeadline(time.Now().Add(s.Timeout))
		s.raw.SetReadDeadline(time.Now().Add(s.Timeout))
	}

	items := make([]*Item, 0, len(keys))
	header := s.reqHeaderPool.Get().(*binaryRequestHeader)
	for _, key := range keys {
		header.Reset()
		header.Opcode = binaryOpcodeGetKQ
		header.KeyLength = uint16(len(key))
		header.TotalBodyLength = uint32(len(key))
		if err := header.EncodeTo(s.conn); err != nil {
			return nil, err
		}
		s.conn.Write([]byte(key))
	}
	s.reqHeaderPool.Put(header)

	header = s.reqHeaderPool.Get().(*binaryRequestHeader)
	header.Reset()
	header.Opcode = binaryOpcodeNoop
	if err := header.EncodeTo(s.conn); err != nil {
		return nil, err
	}
	s.reqHeaderPool.Put(header)
	if err := s.conn.Flush(); err != nil {
		return nil, err
	}

	resHeader := s.resHeaderPool.Get().(*binaryResponseHeader)
	for {
		if err := resHeader.Read(s.conn); err != nil {
			return nil, err
		}
		if resHeader.Opcode == binaryOpcodeNoop {
			break
		}

		if resHeader.Status != 0 {
			if v, ok := binaryStatus[resHeader.Status]; ok {
				return nil, fmt.Errorf("memcached: error %s", v)
			}
			return nil, fmt.Errorf("memcached: unknown error %d", resHeader.Status)
		}

		buf := make([]byte, resHeader.TotalBodyLength)
		if _, err := io.ReadFull(s.conn, buf); err != nil {
			return nil, err
		}

		items = append(items, &Item{
			Key:   string(buf[resHeader.ExtraLength : uint16(resHeader.ExtraLength)+resHeader.KeyLength]),
			Value: buf[uint16(resHeader.ExtraLength)+resHeader.KeyLength:],
			Flags: buf[:resHeader.ExtraLength],
		})
	}

	return items, nil
}

func (s *ServerWithBinaryProtocol) Set(item *Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Timeout != 0 {
		s.raw.SetWriteDeadline(time.Now().Add(s.Timeout))
		s.raw.SetReadDeadline(time.Now().Add(s.Timeout))
	}

	header := s.getReqHeader()
	defer s.putReqHeader(header)

	header.Opcode = binaryOpcodeSet
	extra := item.marshalBinaryRequestHeader(header)
	if err := header.EncodeTo(s.conn); err != nil {
		return err
	}
	if item.Flags != nil {
		copy(extra[:4], item.Flags[:4])
	}
	s.conn.Write(extra)
	s.conn.Write([]byte(item.Key))
	s.conn.Write(item.Value)
	if err := s.conn.Flush(); err != nil {
		return err
	}

	resHeader := s.resHeaderPool.Get().(*binaryResponseHeader)
	defer s.resHeaderPool.Put(resHeader)
	if err := resHeader.Read(s.conn); err != nil {
		return err
	}
	if resHeader.Opcode != binaryOpcodeSet {
		return errors.New("memcached: invalid response")
	}

	if resHeader.Status != 0 {
		switch resHeader.Status {
		case 2: // key exists
			return merrors.ItemExists
		default:
			if v, ok := binaryStatus[resHeader.Status]; ok {
				return fmt.Errorf("memcached: error %s", v)
			} else {
				return fmt.Errorf("memcached: unknown error %d", resHeader.Status)
			}
		}
	}

	return nil
}

func (s *ServerWithBinaryProtocol) Add(item *Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Timeout != 0 {
		s.raw.SetWriteDeadline(time.Now().Add(s.Timeout))
		s.raw.SetReadDeadline(time.Now().Add(s.Timeout))
	}

	header := s.getReqHeader()
	defer s.putReqHeader(header)

	header.Opcode = binaryOpcodeAdd
	extra := item.marshalBinaryRequestHeader(header)
	if err := header.EncodeTo(s.conn); err != nil {
		return err
	}
	if item.Flags != nil {
		copy(extra[:4], item.Flags[:4])
	}
	s.conn.Write(extra)
	s.conn.Write([]byte(item.Key))
	s.conn.Write(item.Value)
	if err := s.conn.Flush(); err != nil {
		return err
	}

	resHeader := s.resHeaderPool.Get().(*binaryResponseHeader)
	defer s.resHeaderPool.Put(resHeader)
	if err := resHeader.Read(s.conn); err != nil {
		return err
	}
	if resHeader.Opcode != binaryOpcodeAdd {
		return errors.New("memcached: invalid response")
	}

	if resHeader.Status != 0 {
		switch resHeader.Status {
		case 2:
			return merrors.ItemExists
		default:
			if v, ok := binaryStatus[resHeader.Status]; ok {
				return fmt.Errorf("memcached: error %s", v)
			} else {
				return fmt.Errorf("memcached: unknown error %d", resHeader.Status)
			}
		}
	}

	return nil
}

func (s *ServerWithBinaryProtocol) Replace(item *Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Timeout != 0 {
		s.raw.SetWriteDeadline(time.Now().Add(s.Timeout))
		s.raw.SetReadDeadline(time.Now().Add(s.Timeout))
	}

	header := s.getReqHeader()
	defer s.putReqHeader(header)

	header.Opcode = binaryOpcodeReplace
	extra := item.marshalBinaryRequestHeader(header)
	if err := header.EncodeTo(s.conn); err != nil {
		return err
	}
	if item.Flags != nil {
		copy(extra[:4], item.Flags[:4])
	}
	s.conn.Write(extra)
	s.conn.Write([]byte(item.Key))
	s.conn.Write(item.Value)
	if err := s.conn.Flush(); err != nil {
		return err
	}

	resHeader := s.resHeaderPool.Get().(*binaryResponseHeader)
	defer s.resHeaderPool.Put(resHeader)
	if err := resHeader.Read(s.conn); err != nil {
		return err
	}
	if resHeader.Opcode != binaryOpcodeReplace {
		return errors.New("memcached: invalid response")
	}

	buf := make([]byte, resHeader.TotalBodyLength)
	if _, err := io.ReadFull(s.conn, buf); err != nil {
		return err
	}

	if resHeader.Status != 0 {
		switch resHeader.Status {
		case 1:
			return merrors.ItemNotFound
		default:
			if v, ok := binaryStatus[resHeader.Status]; ok {
				return fmt.Errorf("memcached: error %s", v)
			} else {
				return fmt.Errorf("memcached: unknown error %d", resHeader.Status)
			}
		}
	}

	return nil
}

func (s *ServerWithBinaryProtocol) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Timeout != 0 {
		s.raw.SetWriteDeadline(time.Now().Add(s.Timeout))
		s.raw.SetReadDeadline(time.Now().Add(s.Timeout))
	}

	header := s.reqHeaderPool.Get().(*binaryRequestHeader)
	defer s.reqHeaderPool.Put(header)
	header.Reset()

	header.Opcode = binaryOpcodeDelete
	header.KeyLength = uint16(len(key))
	header.TotalBodyLength = uint32(len(key))
	if err := header.EncodeTo(s.conn); err != nil {
		return err
	}
	s.conn.Write([]byte(key))
	if err := s.conn.Flush(); err != nil {
		return err
	}

	resHeader := s.resHeaderPool.Get().(*binaryResponseHeader)
	defer s.resHeaderPool.Put(resHeader)
	if err := resHeader.Read(s.conn); err != nil {
		return err
	}
	if resHeader.Opcode != binaryOpcodeDelete {
		return errors.New("memcached: invalid response")
	}

	buf := make([]byte, resHeader.TotalBodyLength)
	if _, err := io.ReadFull(s.conn, buf); err != nil {
		return err
	}

	if resHeader.Status != 0 {
		switch resHeader.Status {
		case 1: // key not found
			return merrors.ItemNotFound
		default:
			if v, ok := binaryStatus[resHeader.Status]; ok {
				return fmt.Errorf("memcached: error %s", v)
			} else {
				return fmt.Errorf("memcached: unknown error %d", resHeader.Status)
			}
		}
	}

	return nil
}

func (s *ServerWithBinaryProtocol) Increment(key string, delta, expiration int) (int64, error) {
	return s.incrOrDecr(binaryOpcodeIncrement, key, delta, expiration)
}

func (s *ServerWithBinaryProtocol) Decrement(key string, delta, expiration int) (int64, error) {
	return s.incrOrDecr(binaryOpcodeDecrement, key, delta, expiration)
}

func (s *ServerWithBinaryProtocol) incrOrDecr(opcode byte, key string, delta int, expiration int) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Timeout != 0 {
		s.raw.SetWriteDeadline(time.Now().Add(s.Timeout))
		s.raw.SetReadDeadline(time.Now().Add(s.Timeout))
	}

	extra := make([]byte, 20)
	binary.BigEndian.PutUint64(extra[:8], uint64(delta))
	binary.BigEndian.PutUint64(extra[8:16], 1)
	binary.BigEndian.PutUint32(extra[16:20], uint32(expiration))
	header := s.reqHeaderPool.Get().(*binaryRequestHeader)
	defer s.reqHeaderPool.Put(header)

	header.Opcode = opcode
	header.KeyLength = uint16(len(key))
	header.ExtraLength = uint8(len(extra))
	header.TotalBodyLength = uint32(len(key) + len(extra))

	if err := header.EncodeTo(s.conn); err != nil {
		return 0, err
	}
	s.conn.Write(extra)
	s.conn.Write([]byte(key))
	if err := s.conn.Flush(); err != nil {
		return 0, err
	}

	resHeader := s.resHeaderPool.Get().(*binaryResponseHeader)
	defer s.resHeaderPool.Put(resHeader)

	if err := resHeader.Read(s.conn); err != nil {
		return 0, err
	}
	if resHeader.Opcode != opcode {
		return 0, errors.New("memcached: invalid response")
	}

	var body []byte
	if resHeader.TotalBodyLength > 0 {
		buf := make([]byte, resHeader.TotalBodyLength)
		if _, err := io.ReadFull(s.conn, buf); err != nil {
			return 0, err
		}
		body = buf
	}

	if resHeader.Status != 0 {
		if v, ok := binaryStatus[resHeader.Status]; ok {
			additional := ""
			if len(body) > 0 {
				additional = " (" + string(body) + ")"
			}
			return 0, fmt.Errorf("memcached: error %s%s", v, additional)
		} else {
			return 0, fmt.Errorf("memcached: unknown error %d (%s)", resHeader.Status, string(body))
		}
	}

	return int64(binary.BigEndian.Uint64(body)), nil
}

func (s *ServerWithBinaryProtocol) Touch(key string, expiration int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Timeout != 0 {
		s.raw.SetWriteDeadline(time.Now().Add(s.Timeout))
		s.raw.SetReadDeadline(time.Now().Add(s.Timeout))
	}

	extra := make([]byte, 4)
	binary.BigEndian.PutUint32(extra, uint32(expiration))
	header := s.reqHeaderPool.Get().(*binaryRequestHeader)
	defer s.reqHeaderPool.Put(header)
	header.Reset()

	header.Opcode = binaryOpcodeTouch
	header.ExtraLength = uint8(len(extra))
	header.KeyLength = uint16(len(key))
	header.TotalBodyLength = uint32(len(key) + len(extra))
	if err := header.EncodeTo(s.conn); err != nil {
		return err
	}
	s.conn.Write(extra)
	s.conn.Write([]byte(key))
	if err := s.conn.Flush(); err != nil {
		return err
	}

	resHeader := s.resHeaderPool.Get().(*binaryResponseHeader)
	defer s.resHeaderPool.Put(resHeader)

	if err := resHeader.Read(s.conn); err != nil {
		return err
	}

	if resHeader.Status != 0 {
		if v, ok := binaryStatus[resHeader.Status]; ok {
			return fmt.Errorf("memcached: error %s", v)
		} else {
			return fmt.Errorf("memcached: unknown error %d", resHeader.Status)
		}
	}

	return nil
}

func (s *ServerWithBinaryProtocol) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Timeout != 0 {
		s.raw.SetWriteDeadline(time.Now().Add(s.Timeout))
		s.raw.SetReadDeadline(time.Now().Add(s.Timeout))
	}

	header := s.getReqHeader()
	defer s.putReqHeader(header)

	header.Opcode = binaryOpcodeFlush
	if err := header.EncodeTo(s.conn); err != nil {
		return err
	}
	if err := s.conn.Flush(); err != nil {
		return err
	}

	resHeader := s.resHeaderPool.Get().(*binaryResponseHeader)
	defer s.resHeaderPool.Put(resHeader)

	if err := resHeader.Read(s.conn); err != nil {
		return err
	}
	if resHeader.Opcode != binaryOpcodeFlush {
		return errors.New("memcached: invalid response")
	}

	return nil
}

func (s *ServerWithBinaryProtocol) Version() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Timeout != 0 {
		s.raw.SetWriteDeadline(time.Now().Add(s.Timeout))
		s.raw.SetReadDeadline(time.Now().Add(s.Timeout))
	}

	header := s.getReqHeader()
	defer s.putReqHeader(header)

	header.Opcode = binaryOpcodeVersion
	if err := header.EncodeTo(s.conn); err != nil {
		return "", err
	}
	if err := s.conn.Flush(); err != nil {
		return "", err
	}

	resHeader := s.resHeaderPool.Get().(*binaryResponseHeader)
	defer s.resHeaderPool.Put(resHeader)
	if err := resHeader.Read(s.conn); err != nil {
		return "", err
	}

	if resHeader.Status != 0 {
		if v, ok := binaryStatus[resHeader.Status]; ok {
			return "", fmt.Errorf("memcached: error %s", v)
		}
		return "", fmt.Errorf("memcached: unknown error %d", resHeader.Status)
	}

	buf := make([]byte, resHeader.TotalBodyLength)
	if _, err := io.ReadFull(s.conn, buf); err != nil {
		return "", err
	}

	return string(buf), nil
}

func (s *ServerWithBinaryProtocol) Close() error {
	return s.raw.Close()
}

func (s *ServerWithBinaryProtocol) getReqHeader() *binaryRequestHeader {
	h := s.reqHeaderPool.Get().(*binaryRequestHeader)
	h.Reset()
	return h
}

func (s *ServerWithBinaryProtocol) putReqHeader(h *binaryRequestHeader) {
	s.reqHeaderPool.Put(h)
}

type binaryRequestHeader struct {
	Opcode          byte
	KeyLength       uint16
	ExtraLength     uint8
	DataType        byte
	VBucketId       uint16
	TotalBodyLength uint32
	Opaque          uint32
	CAS             uint64

	buf []byte
}

func newBinaryRequestHeader() *binaryRequestHeader {
	return &binaryRequestHeader{buf: make([]byte, 24)}
}

func (h *binaryRequestHeader) EncodeTo(w io.Writer) error {
	h.buf[0] = magicRequest
	h.buf[1] = h.Opcode
	binary.BigEndian.PutUint16(h.buf[2:4], h.KeyLength)        // ken len
	h.buf[4] = h.ExtraLength                                   // extra len
	h.buf[5] = h.DataType                                      // data type
	binary.BigEndian.PutUint16(h.buf[6:8], 0)                  // vbucket id
	binary.BigEndian.PutUint32(h.buf[8:12], h.TotalBodyLength) // total body len
	binary.BigEndian.PutUint32(h.buf[12:16], 0)                // opaque
	binary.BigEndian.PutUint64(h.buf[16:24], h.CAS)            // cas

	if _, err := w.Write(h.buf); err != nil {
		return err
	}
	return nil
}

func (h *binaryRequestHeader) Reset() {
	h.Opcode = 0
	h.KeyLength = 0
	h.ExtraLength = 0
	h.DataType = 0
	h.VBucketId = 0
	h.TotalBodyLength = 0
	h.Opaque = 0
	h.CAS = 0
}

type binaryResponseHeader struct {
	Opcode          byte
	KeyLength       uint16
	ExtraLength     uint8
	DataType        byte
	Status          uint16
	TotalBodyLength uint32
	Opaque          uint32
	CAS             uint64

	buf []byte
}

func newBinaryResponseHeader() *binaryResponseHeader {
	return &binaryResponseHeader{buf: make([]byte, 24)}
}

func (h *binaryResponseHeader) Read(r io.Reader) error {
	if _, err := io.ReadFull(r, h.buf); err != nil {
		return err
	}
	if h.buf[0] != magicResponse {
		return errors.New("memcached: invalid response")
	}
	h.Opcode = h.buf[1]
	h.KeyLength = binary.BigEndian.Uint16(h.buf[2:4])
	h.ExtraLength = h.buf[4]
	h.DataType = h.buf[5]
	h.Status = binary.BigEndian.Uint16(h.buf[6:8])
	h.TotalBodyLength = binary.BigEndian.Uint32(h.buf[8:12])
	h.Opaque = binary.BigEndian.Uint32(h.buf[12:16])
	h.CAS = binary.BigEndian.Uint64(h.buf[16:24])

	return nil
}
