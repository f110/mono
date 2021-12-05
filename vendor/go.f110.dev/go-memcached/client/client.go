package client

import (
	"encoding/binary"
)

type Item struct {
	Key        string
	Value      []byte
	Flags      []byte
	Expiration int
	Cas        uint64

	Server string
}

func (i *Item) marshalBinaryRequestHeader(h *binaryRequestHeader) []byte {
	extra := make([]byte, 8)
	binary.BigEndian.PutUint32(extra[4:8], uint32(i.Expiration))
	h.KeyLength = uint16(len(i.Key))
	h.ExtraLength = uint8(len(extra))
	h.TotalBodyLength = uint32(len(i.Key) + len(i.Value) + len(extra))
	h.CAS = i.Cas
	return extra
}

type SinglePool struct {
	ring *Ring
}

func NewSinglePool(servers ...Server) (*SinglePool, error) {
	ring := NewRing(servers...)

	return &SinglePool{ring: ring}, nil
}

func (c *SinglePool) Get(key string) (*Item, error) {
	return c.ring.Pick(key).Get(key)
}

func (c *SinglePool) Set(item *Item) error {
	return c.ring.Pick(item.Key).Set(item)
}

func (c *SinglePool) Add(item *Item) error {
	return c.ring.Pick(item.Key).Add(item)
}

func (c *SinglePool) Replace(item *Item) error {
	return c.ring.Pick(item.Key).Replace(item)
}

func (c *SinglePool) GetMulti(keys ...string) ([]*Item, error) {
	keyMap := make(map[string][]string)
	for _, key := range keys {
		s := c.ring.Pick(key)
		if _, ok := keyMap[s.Name()]; !ok {
			keyMap[s.Name()] = make([]string, 0)
		}
		keyMap[s.Name()] = append(keyMap[s.Name()], key)
	}

	result := make([]*Item, 0, len(keys))
	for serverName, keys := range keyMap {
		s := c.ring.Find(serverName)
		items, err := s.GetMulti(keys...)
		if err != nil {
			return nil, err
		}
		result = append(result, items...)
	}
	return result, nil
}

func (c *SinglePool) Delete(key string) error {
	return c.ring.Pick(key).Delete(key)
}

func (c *SinglePool) Increment(key string, delta, expiration int) (int64, error) {
	return c.ring.Pick(key).Increment(key, delta, expiration)
}

func (c *SinglePool) Decrement(key string, delta, expiration int) (int64, error) {
	return c.ring.Pick(key).Decrement(key, delta, expiration)
}

func (c *SinglePool) Touch(key string, expiration int) error {
	return c.ring.Pick(key).Touch(key, expiration)
}

func (c *SinglePool) Flush() error {
	return c.ring.Each(func(s Server) error {
		return s.Flush()
	})
}

func (c *SinglePool) Version() (map[string]string, error) {
	result := make(map[string]string)
	err := c.ring.Each(func(s Server) error {
		if v, err := s.Version(); err != nil {
			return err
		} else {
			result[s.Name()] = v
		}
		return nil
	})

	return result, err
}

func (c *SinglePool) Close() error {
	return c.ring.Each(func(s Server) error {
		return s.Close()
	})
}
