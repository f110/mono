package client

import (
	"crypto/sha1"
	"encoding/binary"
	"hash/crc32"
	"sort"
	"strconv"
)

const (
	numberOfDivideServer = 200
)

type Node struct {
	Server Server
	hash   uint32
}

type Ring struct {
	nodes   []*Node
	servers map[string]Server
}

func NewRing(servers ...Server) *Ring {
	nodes := make([]*Node, 0, len(servers)+numberOfDivideServer)
	for _, v := range servers {
		for i := 0; i < numberOfDivideServer; i++ {
			s := sha1.Sum([]byte(v.Name() + "/" + strconv.Itoa(i)))
			nodes = append(nodes, &Node{
				Server: v,
				hash:   binary.BigEndian.Uint32(s[:4]),
			})
		}
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].hash < nodes[j].hash
	})

	s := make(map[string]Server)
	for _, v := range servers {
		s[v.Name()] = v
	}

	return &Ring{
		nodes:   nodes,
		servers: s,
	}
}

func (r *Ring) Pick(key string) Server {
	h := crc32.ChecksumIEEE([]byte(key))

	lower := 0
	upper := len(r.nodes) - 1
Search:
	for lower <= upper {
		idx := (lower + upper) / 2

		t := r.nodes[idx].hash
		switch {
		case t == h:
			break Search
		case h < t:
			upper = idx - 1
		case t < h:
			lower = idx + 1
		}
	}

	return r.nodes[upper].Server
}

func (r *Ring) Find(name string) Server {
	return r.servers[name]
}

func (r *Ring) Each(fn func(s Server) error) error {
	for _, s := range r.servers {
		if err := fn(s); err != nil {
			return err
		}
	}

	return nil
}

func (r *Ring) Close() error {
	var e error
	for _, v := range r.servers {
		if err := v.Close(); err != nil {
			e = err
		}
	}

	return e
}
