package xnet

import (
	"fmt"
	"sync/atomic"
	"time"
)

type Session interface {
	Cryptor
	ECDHable
	Metadata
	RouteTableRenewal

	StartTime() int64
	CSIndex() int32
	IncreaseCSIndex() int32
}

// Cryptor is the interface for the cryptor.
type Cryptor interface {
	Key() []byte
	IsCrypto() bool
	Encrypt(data Pack) (Pack, error)
	Decrypt(data Pack) (Pack, error)
}

type RouteTableRenewal interface {
	NextRenewTime() time.Time
	UpdateNextRenewTime(t time.Time)
}

type Metadata interface {
	UID() int64
	SID() int64
	Color() string
	Status() int64
	ClientIP() string
	SetClientIP(ip string)
	LogInfo() string
}

var _ Session = (*session)(nil)

type session struct {
	Cryptor
	ECDHable

	uid         int64
	sid         int64
	color       string
	status      int64
	clientIP    string
	startTime   int64
	csIndex     *indexInfo
	nextRenewAt time.Time
}

// DefaultSession creates a new session with default values.
func DefaultSession() Session {
	return &session{
		Cryptor:   NewUnCryptor(),
		ECDHable:  NewUnECDH(),
		csIndex:   newIndexInfo(0),
		startTime: time.Now().Unix(),
	}
}

type Option func(s *session)

func WithSID(sid int64) Option {
	return func(s *session) {
		s.sid = sid
	}
}

func WithEncryptor(encryptor Cryptor) Option {
	return func(s *session) {
		s.Cryptor = encryptor
	}
}

func WithECDH(ecdh ECDHable) Option {
	return func(s *session) {
		s.ECDHable = ecdh
	}
}

func WithStartTime(st int64) Option {
	return func(s *session) {
		s.startTime = st
	}
}

func WithCSIndex(i int32) Option {
	return func(s *session) {
		s.csIndex = newIndexInfo(i)
	}
}

// NewSession creates a new session.
//
// userId: the user id of the session.
// color: the color of the session.
// status: the status of the session.
// opts: the options of the session.
func NewSession(userId int64, color string, status int64, opts ...Option) Session {
	s := DefaultSession().(*session)

	for _, opt := range opts {
		opt(s)
	}

	s.uid = userId
	s.color = color
	s.status = status

	return s
}

func (s *session) IncreaseCSIndex() int32 {
	return s.csIndex.Increase()
}

func (s *session) CSIndex() int32 {
	return s.csIndex.Load()
}

func (s *session) StartTime() int64 {
	return s.startTime
}

func (s *session) UID() int64 {
	return s.uid
}

func (s *session) SID() int64 {
	return s.sid
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

func (s *session) NextRenewTime() time.Time {
	return s.nextRenewAt
}

func (s *session) UpdateNextRenewTime(t time.Time) {
	s.nextRenewAt = t
}

func (s *session) LogInfo() string {
	return fmt.Sprintf("uid=%d sid=%d color=%s status=%d ip=%s", s.UID(), s.SID(), s.Color(), s.Status(), s.ClientIP())
}

type indexInfo struct {
	start int32
	index atomic.Int32
}

func newIndexInfo(start int32) *indexInfo {
	i := &indexInfo{
		start: start,
	}
	i.index.Store(start)

	return i
}

func (i *indexInfo) Increase() int32 {
	return i.index.Add(1)
}

func (i *indexInfo) Load() int32 {
	return i.index.Load()
}
