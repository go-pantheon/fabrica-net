package xnet

import (
	"github.com/go-pantheon/fabrica-util/security/ecdh"
)

type ECDHable interface {
	TargetPublicKey() []byte
	ServerPublicKey() []byte
	SharedKey() []byte
}

var _ ECDHable = (*ecdhInfo)(nil)

type ecdhInfo struct {
	selfPri   [32]byte
	selfPub   [32]byte
	targetPub [32]byte
	sharedKey []byte
}

func NewUnECDH() ECDHable {
	return &ecdhInfo{
		selfPri:   [32]byte{},
		selfPub:   [32]byte{},
		targetPub: [32]byte{},
		sharedKey: []byte{},
	}
}

func NewECDHInfo(targetPub [32]byte) (ECDHable, error) {
	selfPri, selfPub, err := ecdh.GenKeyPair()
	if err != nil {
		return nil, err
	}

	sharedKey, err := ecdh.ComputeSharedKey(selfPri, targetPub)
	if err != nil {
		return nil, err
	}

	return &ecdhInfo{
		selfPri:   selfPri,
		selfPub:   selfPub,
		targetPub: targetPub,
		sharedKey: sharedKey,
	}, nil
}

func (e *ecdhInfo) TargetPublicKey() []byte {
	return ecdh.KeyToBytes(&e.targetPub)
}

func (e *ecdhInfo) ServerPublicKey() []byte {
	return ecdh.KeyToBytes(&e.selfPub)
}

func (e *ecdhInfo) SharedKey() []byte {
	return e.sharedKey
}
