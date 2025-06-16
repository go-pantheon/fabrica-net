package xnet

import (
	"sync/atomic"
)

type Session interface {
	Cryptor
	ECDHable

	UID() int64
	SID() int64
	Color() string
	Status() int64
	StartTime() int64

	ClientIP() string
	SetClientIP(ip string)

	CSIndex() int64
	IncreaseCSIndex() int64
}

// Cryptor is the interface for the cryptor.
type Cryptor interface {
	Key() []byte
	IsCrypto() bool
	Encrypt(data Pack) (Pack, error)
	Decrypt(data Pack) (Pack, error)
}

type ECDHable interface {
	ClientPublicKey() []byte
	ServerPublicKey() []byte
	SharedKey() []byte
}

var _ Session = (*session)(nil)

type session struct {
	Cryptor
	ECDHable

	userID    int64
	serverID  int64
	clientIP  string
	color     string
	status    int64
	startTime int64
	csIndex   *indexInfo
}

// DefaultSession creates a new session with default values.
func DefaultSession() Session {
	return &session{
		Cryptor:  NewUnCryptor(),
		ECDHable: NewUnECDH(),
		csIndex:  newIndexInfo(0),
	}
}

// NewSession creates a new session.
//
// userId: the user id of the session.
// sid: the server id of the session.
// st: the start time of the session.
// encryptor: the encryptor of the session.
// color: the color of the session.
func NewSession(userId int64, sid int64, st int64, encryptor Cryptor, ecdh ECDHable, color string, status int64) Session {
	s := &session{
		Cryptor:   encryptor,
		ECDHable:  ecdh,
		userID:    userId,
		color:     color,
		status:    status,
		serverID:  sid,
		startTime: st,
		csIndex:   newIndexInfo(0),
	}

	return s
}

func (s *session) IncreaseCSIndex() int64 {
	return s.csIndex.Increase()
}

func (s *session) CSIndex() int64 {
	return s.csIndex.Load()
}

func (s *session) StartTime() int64 {
	return s.startTime
}

func (s *session) UID() int64 {
	return s.userID
}

func (s *session) SID() int64 {
	return s.serverID
}

func (s *session) Color() string {
	if len(s.color) == 0 {
		return ""
	}

	return s.color
}

func (s *session) Status() int64 {
	return s.status
}

func (s *session) ClientIP() string {
	return s.clientIP
}

func (s *session) SetClientIP(ip string) {
	s.clientIP = ip
}

type indexInfo struct {
	start int64
	index atomic.Int64
}

func newIndexInfo(start int64) *indexInfo {
	i := &indexInfo{
		start: start,
	}
	i.index.Store(start)

	return i
}

func (i *indexInfo) Increase() int64 {
	return i.index.Add(1)
}

func (i *indexInfo) Load() int64 {
	return i.index.Load()
}
