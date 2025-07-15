package message

import (
	"fmt"

	"github.com/go-pantheon/fabrica-net/xnet"
)

const (
	ModAuth = iota + 1
	ModEcho
)

var _ xnet.TunnelMessage = (*Packet)(nil)
var _ xnet.PacketMessage = (*Packet)(nil)

type TunnelMessage Packet

var NewTunnelMessage = NewPacket

type Packet struct {
	Mod         int32  `json:"mod"`
	Seq         int32  `json:"seq"`
	Obj         int64  `json:"obj"`
	Index       int32  `json:"index"`
	Data        []byte `json:"data"`
	DataVersion uint64 `json:"data_version"`
	ConnID      int32  `json:"conn_id"`
	FragID      int32  `json:"frag_id"`
	FragCount   int32  `json:"frag_count"`
	FragIndex   int32  `json:"frag_index"`
}

func NewPacket(mod int32, seq int32, obj int64, index int32, data []byte, dataVersion uint64) *Packet {
	return &Packet{
		Mod:         mod,
		Seq:         seq,
		Obj:         obj,
		Index:       index,
		Data:        data,
		DataVersion: dataVersion,
	}
}

func (m *Packet) GetMod() int32 {
	return m.Mod
}

func (m *Packet) GetSeq() int32 {
	return m.Seq
}

func (m *Packet) GetObj() int64 {
	return m.Obj
}

func (m *Packet) GetData() []byte {
	return m.Data
}

func (m *Packet) GetIndex() int32 {
	return m.Index
}

func (m *Packet) GetDataVersion() uint64 {
	return m.DataVersion
}

func (m *Packet) GetConnId() int32 {
	return m.ConnID
}

func (m *Packet) GetFragId() int32 {
	return m.FragID
}

func (m *Packet) GetFragCount() int32 {
	return m.FragCount
}

func (m *Packet) GetFragIndex() int32 {
	return m.FragIndex
}

func (m *Packet) String() string {
	return fmt.Sprintf("mod=%d seq=%d obj=%d index=%d data=%s data_version=%d", m.Mod, m.Seq, m.Obj, m.Index, string(m.Data), m.DataVersion)
}
